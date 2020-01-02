package accesslog

import (
	"net/http"
	"sync"
)

type captureRequestReader struct {
	req   *http.Request
	count int64
	mu    sync.Mutex
	done  bool
}

func (r *captureRequestReader) waitForClosure() {
	select {
	case <-r.req.Context().Done():
		r.mu.Lock()
		r.done = true
		r.mu.Unlock()
	}
}

func (r *captureRequestReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.done {
		return 0, http.ErrBodyReadAfterClose
	}

	n, err := r.req.Body.Read(p)
	r.count += int64(n)
	return n, err
}

func (r *captureRequestReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.done {
		return http.ErrBodyReadAfterClose
	}

	return r.req.Body.Close()
}
