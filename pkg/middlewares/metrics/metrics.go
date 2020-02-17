package metrics

import (
	"context"
	"crypto/tls"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/containous/alice"
	"github.com/containous/traefik/v2/pkg/log"
	"github.com/containous/traefik/v2/pkg/metrics"
	"github.com/containous/traefik/v2/pkg/middlewares"
	"github.com/containous/traefik/v2/pkg/middlewares/retry"
	gokitmetrics "github.com/go-kit/kit/metrics"
)

const (
	protoHTTP      = "http"
	protoSSE       = "sse"
	protoWebsocket = "websocket"
	typeName       = "Metrics"
	nameEntrypoint = "metrics-entrypoint"
	nameService    = "metrics-service"
)

type metricsMiddleware struct {
	// Important: Since this int64 field is using sync/atomic, it has to be at the top of the struct due to a bug on 32-bit platform
	// See: https://golang.org/pkg/sync/atomic/ for more information
	openConns            int64
	next                 http.Handler
	reqsCounter          gokitmetrics.Counter
	reqsTLSCounter       gokitmetrics.Counter
	reqDurationHistogram gokitmetrics.Histogram
	openConnsGauge       gokitmetrics.Gauge
	baseLabels           []string
}

// NewEntryPointMiddleware creates a new metrics middleware for an Entrypoint.
func NewEntryPointMiddleware(ctx context.Context, next http.Handler, registry metrics.Registry, entryPointName string) http.Handler {
	log.FromContext(middlewares.GetLoggerCtx(ctx, nameEntrypoint, typeName)).Debug("Creating middleware")

	return &metricsMiddleware{
		next:                 next,
		reqsCounter:          registry.EntryPointReqsCounter(),
		reqsTLSCounter:       registry.EntryPointReqsTLSCounter(),
		reqDurationHistogram: registry.EntryPointReqDurationHistogram(),
		openConnsGauge:       registry.EntryPointOpenConnsGauge(),
		baseLabels:           []string{"entrypoint", entryPointName},
	}
}

// NewServiceMiddleware creates a new metrics middleware for a Service.
func NewServiceMiddleware(ctx context.Context, next http.Handler, registry metrics.Registry, serviceName string) http.Handler {
	log.FromContext(middlewares.GetLoggerCtx(ctx, nameService, typeName)).Debug("Creating middleware")

	return &metricsMiddleware{
		next:                 next,
		reqsCounter:          registry.ServiceReqsCounter(),
		reqsTLSCounter:       registry.ServiceReqsTLSCounter(),
		reqDurationHistogram: registry.ServiceReqDurationHistogram(),
		openConnsGauge:       registry.ServiceOpenConnsGauge(),
		baseLabels:           []string{"service", serviceName},
	}
}

// WrapEntryPointHandler Wraps metrics entrypoint to alice.Constructor.
func WrapEntryPointHandler(ctx context.Context, registry metrics.Registry, entryPointName string) alice.Constructor {
	return func(next http.Handler) (http.Handler, error) {
		return NewEntryPointMiddleware(ctx, next, registry, entryPointName), nil
	}
}

// WrapServiceHandler Wraps metrics service to alice.Constructor.
func WrapServiceHandler(ctx context.Context, registry metrics.Registry, serviceName string) alice.Constructor {
	return func(next http.Handler) (http.Handler, error) {
		return NewServiceMiddleware(ctx, next, registry, serviceName), nil
	}
}

func (m *metricsMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	labels := []string{"method", getMethod(req), "protocol", getRequestProtocol(req)}
	labels = append(labels, m.baseLabels...)

	openConns := atomic.AddInt64(&m.openConns, 1)
	m.openConnsGauge.With(labels...).Set(float64(openConns))
	defer func(labelValues []string) {
		openConns := atomic.AddInt64(&m.openConns, -1)
		m.openConnsGauge.With(labelValues...).Set(float64(openConns))
	}(labels)

	// TLS metrics
	if req.TLS != nil {
		tlsLabels := []string{"tls_version", getRequestTLSVersion(req), "tls_cipher", getRequestTLSCipher(req)}
		tlsLabels = append(tlsLabels, m.baseLabels...)

		m.reqsTLSCounter.With(tlsLabels...).Add(1)
	}

	start := time.Now()
	recorder := newResponseRecorder(rw)
	m.next.ServeHTTP(recorder, req)

	labels = append(labels, "code", strconv.Itoa(recorder.getCode()))
	m.reqsCounter.With(labels...).Add(1)
	m.reqDurationHistogram.With(labels...).Observe(time.Since(start).Seconds())
}

func getRequestProtocol(req *http.Request) string {
	switch {
	case isWebsocketRequest(req):
		return protoWebsocket
	case isSSERequest(req):
		return protoSSE
	default:
		return protoHTTP
	}
}

// isWebsocketRequest determines if the specified HTTP request is a websocket handshake request.
func isWebsocketRequest(req *http.Request) bool {
	return containsHeader(req, "Connection", "upgrade") && containsHeader(req, "Upgrade", "websocket")
}

// isSSERequest determines if the specified HTTP request is a request for an event subscription.
func isSSERequest(req *http.Request) bool {
	return containsHeader(req, "Accept", "text/event-stream")
}

func containsHeader(req *http.Request, name, value string) bool {
	items := strings.Split(req.Header.Get(name), ",")
	for _, item := range items {
		if value == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func getMethod(r *http.Request) string {
	if !utf8.ValidString(r.Method) {
		log.Warnf("Invalid HTTP method encoding: %s", r.Method)
		return "NON_UTF8_HTTP_METHOD"
	}
	return r.Method
}

func getRequestTLSVersion(req *http.Request) string {
	var tlsVersion string

	switch req.TLS.Version {
	case tls.VersionTLS10:
		tlsVersion = "tls-1.0"
	case tls.VersionTLS11:
		tlsVersion = "tls-1.1"
	case tls.VersionTLS12:
		tlsVersion = "tls-1.2"
	case tls.VersionTLS13:
		tlsVersion = "tls-1.3"
	case tls.VersionSSL30: //nolint
		tlsVersion = "ssl-3.0"
	default:
		tlsVersion = "unknown"
	}

	return tlsVersion
}

func getRequestTLSCipher(req *http.Request) string {
	var tlsCipher string

	switch req.TLS.CipherSuite {
	case tls.TLS_RSA_WITH_RC4_128_SHA:
		tlsCipher = "TLS_RSA_WITH_RC4_128_SHA"
	case tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA:
		tlsCipher = "TLS_RSA_WITH_3DES_EDE_CBC_SHA"
	case tls.TLS_RSA_WITH_AES_128_CBC_SHA:
		tlsCipher = "TLS_RSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_RSA_WITH_AES_256_CBC_SHA:
		tlsCipher = "TLS_RSA_WITH_AES_256_CBC_SHA"
	case tls.TLS_RSA_WITH_AES_128_CBC_SHA256:
		tlsCipher = "TLS_RSA_WITH_AES_128_CBC_SHA256"
	case tls.TLS_RSA_WITH_AES_128_GCM_SHA256:
		tlsCipher = "TLS_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_RSA_WITH_AES_256_GCM_SHA384:
		tlsCipher = "TLS_RSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_RC4_128_SHA"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA"
	case tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA:
		tlsCipher = "TLS_ECDHE_RSA_WITH_RC4_128_SHA"
	case tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA:
		tlsCipher = "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA:
		tlsCipher = "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA:
		tlsCipher = "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256:
		tlsCipher = "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:
		tlsCipher = "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
	case tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384:
		tlsCipher = "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
	case tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305:
		tlsCipher = "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305"
	case tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305:
		tlsCipher = "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305"
	case tls.TLS_AES_128_GCM_SHA256:
		tlsCipher = "TLS_AES_128_GCM_SHA256"
	case tls.TLS_AES_256_GCM_SHA384:
		tlsCipher = "TLS_AES_256_GCM_SHA384"
	case tls.TLS_CHACHA20_POLY1305_SHA256:
		tlsCipher = "TLS_CHACHA20_POLY1305_SHA256"
	default:
		tlsCipher = "unknown"
	}

	return tlsCipher
}

type retryMetrics interface {
	ServiceRetriesCounter() gokitmetrics.Counter
}

// NewRetryListener instantiates a MetricsRetryListener with the given retryMetrics.
func NewRetryListener(retryMetrics retryMetrics, serviceName string) retry.Listener {
	return &RetryListener{retryMetrics: retryMetrics, serviceName: serviceName}
}

// RetryListener is an implementation of the RetryListener interface to
// record RequestMetrics about retry attempts.
type RetryListener struct {
	retryMetrics retryMetrics
	serviceName  string
}

// Retried tracks the retry in the RequestMetrics implementation.
func (m *RetryListener) Retried(req *http.Request, attempt int) {
	m.retryMetrics.ServiceRetriesCounter().With("service", m.serviceName).Add(1)
}
