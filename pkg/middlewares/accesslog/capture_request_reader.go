package accesslog

import (
	"net/http"
	"sync"
)

type captureRequestReader struct {
	req   *http.Request
	count int64
	mu    sync.Mutex
	// done is a boolean which aim to protect against panics which can occur when
	// we try to read the request body after a connection has been closed. It is
	// set true when req.Context().Done() channel returns a value.
	done bool
}

// waitForClosure is a function which should be ran as a goroutine.
// It reads r.req.Context().Done() and sets r.done to true when a value is returned.
func (r *captureRequestReader) waitForClosure() {
	select {
	case <-r.req.Context().Done():
		r.mu.Lock()
		r.done = true
		r.mu.Unlock()
	}
}

func (r *captureRequestReader) GetCount() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.count
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
