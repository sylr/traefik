package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/containous/traefik/pkg/config/dynamic"
	"github.com/containous/traefik/pkg/log"
	"github.com/containous/traefik/pkg/safe"
	"github.com/go-kit/kit/metrics"
	"github.com/vulcand/oxy/roundrobin"
)

const (
	serverUp   = "UP"
	serverDown = "DOWN"
)

var singleton *HealthCheck
var once sync.Once

// BalancerHandler includes functionality for load-balancing management.
type BalancerHandler interface {
	ServeHTTP(w http.ResponseWriter, req *http.Request)
	Servers() []*url.URL
	RemoveServer(u *url.URL) error
	UpsertServer(u *url.URL, options ...roundrobin.ServerOption) error
}

// metricsRegistry is a local interface in the health check package, exposing only the required metrics
// necessary for the health check package. This makes it easier for the tests.
type metricsRegistry interface {
	BackendServerUpGauge() metrics.Gauge
}

// Options are the public health check options.
type Options struct {
	Headers   map[string]string
	Hostname  string
	Scheme    string
	Path      string
	Port      int
	Transport http.RoundTripper
	Interval  time.Duration
	Timeout   time.Duration
	LB        BalancerHandler
}

func (opt Options) String() string {
	return fmt.Sprintf("[Hostname: %s Headers: %v Path: %s Port: %d Interval: %s Timeout: %s]", opt.Hostname, opt.Headers, opt.Path, opt.Port, opt.Interval, opt.Timeout)
}

// BackendConfig HealthCheck configuration for a backend
type BackendConfig struct {
	Options
	name         string
	disabledURLs []*url.URL
}

func (b *BackendConfig) newRequest(serverURL *url.URL) (*http.Request, error) {
	u, err := serverURL.Parse(b.Path)
	if err != nil {
		return nil, err
	}

	if len(b.Scheme) > 0 {
		u.Scheme = b.Scheme
	}

	if b.Port != 0 {
		u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(b.Port))
	}

	return http.NewRequest(http.MethodGet, u.String(), http.NoBody)
}

// this function adds additional http headers and hostname to http.request
func (b *BackendConfig) addHeadersAndHost(req *http.Request) *http.Request {
	if b.Options.Hostname != "" {
		req.Host = b.Options.Hostname
	}

	for k, v := range b.Options.Headers {
		req.Header.Set(k, v)
	}
	return req
}

// HealthCheck struct
type HealthCheck struct {
	Backends map[string]*BackendConfig
	metrics  metricsRegistry
	cancel   context.CancelFunc
}

// SetBackendsConfiguration set backends configuration
func (hc *HealthCheck) SetBackendsConfiguration(parentCtx context.Context, backends map[string]*BackendConfig) {
	hc.Backends = backends
	if hc.cancel != nil {
		hc.cancel()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	hc.cancel = cancel

	for _, backend := range backends {
		currentBackend := backend
		safe.Go(func() {
			hc.execute(ctx, currentBackend)
		})
	}
}

func (hc *HealthCheck) execute(ctx context.Context, backend *BackendConfig) {
	log.Debugf("Initial health check for backend: %q", backend.name)
	hc.checkBackend(backend)
	ticker := time.NewTicker(backend.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Debugf("Stopping current health check goroutines of backend: %s", backend.name)
			return
		case <-ticker.C:
			log.Debugf("Refreshing health check for backend: %s", backend.name)
			hc.checkBackend(backend)
		}
	}
}

func (hc *HealthCheck) checkBackend(backend *BackendConfig) {
	enabledURLs := backend.LB.Servers()
	var newDisabledURLs []*url.URL
	// FIXME re enable metrics
	for _, disableURL := range backend.disabledURLs {
		// FIXME serverUpMetricValue := float64(0)
		if err := checkHealth(disableURL, backend); err == nil {
			log.Warnf("Health check up: Returning to server list. Backend: %q URL: %q", backend.name, disableURL.String())
			if err = backend.LB.UpsertServer(disableURL, roundrobin.Weight(1)); err != nil {
				log.Error(err)
			}
			// FIXME serverUpMetricValue = 1
		} else {
			log.Warnf("Health check still failing. Backend: %q URL: %q Reason: %s", backend.name, disableURL.String(), err)
			newDisabledURLs = append(newDisabledURLs, disableURL)
		}
		// FIXME labelValues := []string{"backend", backend.name, "url", disableURL.String()}
		// FIXME hc.metrics.BackendServerUpGauge().With(labelValues...).Set(serverUpMetricValue)
	}
	backend.disabledURLs = newDisabledURLs

	// FIXME re enable metrics
	for _, enableURL := range enabledURLs {
		// FIXME serverUpMetricValue := float64(1)
		if err := checkHealth(enableURL, backend); err != nil {
			log.Warnf("Health check failed: Remove from server list. Backend: %q URL: %q Reason: %s", backend.name, enableURL.String(), err)
			if err := backend.LB.RemoveServer(enableURL); err != nil {
				log.Error(err)
			}
			backend.disabledURLs = append(backend.disabledURLs, enableURL)
			// FIXME serverUpMetricValue = 0
		}
		// FIXME labelValues := []string{"backend", backend.name, "url", enableURL.String()}
		// FIXME hc.metrics.BackendServerUpGauge().With(labelValues...).Set(serverUpMetricValue)
	}
}

// FIXME re add metrics
//func GetHealthCheck(metrics metricsRegistry) *HealthCheck {

// GetHealthCheck returns the health check which is guaranteed to be a singleton.
func GetHealthCheck() *HealthCheck {
	once.Do(func() {
		singleton = newHealthCheck()
		//singleton = newHealthCheck(metrics)
	})
	return singleton
}

// FIXME re add metrics
//func newHealthCheck(metrics metricsRegistry) *HealthCheck {
func newHealthCheck() *HealthCheck {
	return &HealthCheck{
		Backends: make(map[string]*BackendConfig),
		//metrics:  metrics,
	}
}

// NewBackendConfig Instantiate a new BackendConfig
func NewBackendConfig(options Options, backendName string) *BackendConfig {
	return &BackendConfig{
		Options: options,
		name:    backendName,
	}
}

// checkHealth returns a nil error in case it was successful and otherwise
// a non-nil error with a meaningful description why the health check failed.
func checkHealth(serverURL *url.URL, backend *BackendConfig) error {
	req, err := backend.newRequest(serverURL)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %s", err)
	}

	req = backend.addHeadersAndHost(req)

	client := http.Client{
		Timeout:   backend.Options.Timeout,
		Transport: backend.Options.Transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("received error status code: %v", resp.StatusCode)
	}

	return nil
}

// NewLBStatusUpdater returns a new LbStatusUpdater
func NewLBStatusUpdater(bh BalancerHandler, svinfo *dynamic.ServiceInfo) *LbStatusUpdater {
	return &LbStatusUpdater{
		BalancerHandler: bh,
		serviceInfo:     svinfo,
	}
}

// LbStatusUpdater wraps a BalancerHandler and a ServiceInfo,
// so it can keep track of the status of a server in the ServiceInfo.
type LbStatusUpdater struct {
	BalancerHandler
	serviceInfo *dynamic.ServiceInfo // can be nil
}

// RemoveServer removes the given server from the BalancerHandler,
// and updates the status of the server to "DOWN".
func (lb *LbStatusUpdater) RemoveServer(u *url.URL) error {
	err := lb.BalancerHandler.RemoveServer(u)
	if err == nil && lb.serviceInfo != nil {
		lb.serviceInfo.UpdateStatus(u.String(), serverDown)
	}
	return err
}

// UpsertServer adds the given server to the BalancerHandler,
// and updates the status of the server to "UP".
func (lb *LbStatusUpdater) UpsertServer(u *url.URL, options ...roundrobin.ServerOption) error {
	err := lb.BalancerHandler.UpsertServer(u, options...)
	if err == nil && lb.serviceInfo != nil {
		lb.serviceInfo.UpdateStatus(u.String(), serverUp)
	}
	return err
}