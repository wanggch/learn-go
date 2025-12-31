package racecase

import (
	"sync"
	"sync/atomic"
)

func UnsafeCounter(n int) int {
	var count int
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			count++
		}()
	}
	wg.Wait()
	return count
}

func SafeCounterMutex(n int) int {
	var count int
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			mu.Lock()
			count++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return count
}

func SafeCounterAtomic(n int) int {
	var count int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			atomic.AddInt64(&count, 1)
		}()
	}
	wg.Wait()
	return int(count)
}
