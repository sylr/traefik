package accesslog

import (
	"errors"
	"io"
)

type captureRequestReader struct {
	// source ReadCloser from where the request body is read.
	source io.ReadCloser
	// count Counts the number of bytes read (when captureRequestReader.Read is called).
	count int64
}

func (r *captureRequestReader) Read(p []byte) (int, error) {
	var err error

	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()

	n, err := r.source.Read(p)
	r.count += int64(n)

	return n, err
}

func (r *captureRequestReader) Close() error {
	var err error

	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("Unknown panic")
			}
		}
	}()

	err = r.source.Close()

	return err
}
