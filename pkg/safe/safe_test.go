package safe

import (
	"runtime"
	"sync"
	"testing"
)

var result int

func TestSafe(t *testing.T) {
	const ts1 = "test1"
	const ts2 = "test2"

	s := NewSync(ts1)

	if result := s.Get(); result != ts1 {
		t.Errorf("Safe.Get() failed, got '%s', expected '%s'", result, ts1)
	}

	s.Set(ts2)

	if result := s.Get(); result != ts2 {
		t.Errorf("Safe.Get() after Safe.Set() failed, got '%s', expected '%s'", result, ts2)
	}
}

func BenchmarkSafeSync(b *testing.B) {
	s := NewSync[int](1)

	var wg sync.WaitGroup

	wg.Add(1)

	// Writing loop
	go func() {
		for i := 0; i < b.N; i++ {
			s.Set(i)
		}

		wg.Done()
	}()

	for g := 0; g < runtime.GOMAXPROCS(0); g++ {
		wg.Add(1)

		go func() {
			for i := 0; i < b.N; i++ {
				result = s.Get()
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

func BenchmarkSafeAtomic(b *testing.B) {
	s := NewAtomic[int](1)

	var wg sync.WaitGroup

	wg.Add(1)

	// Writing loop
	go func() {
		for i := 0; i < b.N; i++ {
			s.Set(i)
		}

		wg.Done()
	}()

	for g := 0; g < runtime.GOMAXPROCS(0); g++ {
		wg.Add(1)

		go func() {
			for i := 0; i < b.N; i++ {
				result = s.Get()
			}

			wg.Done()
		}()
	}

	wg.Wait()
}
