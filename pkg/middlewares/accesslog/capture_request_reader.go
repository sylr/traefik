package accesslog

import (
	"io"
	"net/http"
	"sync"
)

type captureRequestReader struct {
	req   *http.Request
	body  io.ReadCloser
	count int64
	mux   sync.Mutex
}

func (r *captureRequestReader) setBody() {
	if r.body == nil {
		r.body, _ = r.req.GetBody()
	}
}

func (r *captureRequestReader) Read(p []byte) (int, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.setBody()

	if r.body != nil {
		n, err := r.body.Read(p)
		r.count += int64(n)
		return n, err
	}

	return 0, nil
}

func (r *captureRequestReader) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()

	r.setBody()

	return r.req.Body.Close()
}
