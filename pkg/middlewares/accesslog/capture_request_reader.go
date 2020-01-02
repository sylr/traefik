package accesslog

import (
	"net/http"
	"sync"
)

type captureRequestReader struct {
	req    *http.Request
	count  int64
	closed bool
	mux    sync.Mutex
}

func (r *captureRequestReader) Read(p []byte) (int, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.closed {
		return 0, nil
	}

	n, err := r.req.Body.Read(p)
	if err != nil {
		return 0, err
	}

	r.count += int64(n)
	return n, err
}

func (r *captureRequestReader) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return r.req.Body.Close()
}
