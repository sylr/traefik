package accesslog

import (
	"net/http"
	"sync"
)

type captureRequestReader struct {
	req   *http.Request
	count int64
	mux   sync.Mutex
	done  bool
}

func (r *captureRequestReader) waitForClosure() {
	select {
	case <-r.req.Context().Done():
		r.mux.Lock()
		r.done = true
		defer r.mux.Unlock()
	}
}

func (r *captureRequestReader) Read(p []byte) (int, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.done {
		return 0, http.ErrBodyReadAfterClose
	}

	n, err := r.req.Body.Read(p)
	r.count += int64(n)
	return n, err
}

func (r *captureRequestReader) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.done {
		return http.ErrBodyReadAfterClose
	}

	return r.req.Body.Close()
}
