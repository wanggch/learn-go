package racecase

import "testing"

func TestUnsafeCounter(t *testing.T) {
	for i := 0; i < 3; i++ {
		_ = UnsafeCounter(1000)
	}
}

func TestSafeCounterMutex(t *testing.T) {
	got := SafeCounterMutex(1000)
	if got != 1000 {
		t.Fatalf("got %d want %d", got, 1000)
	}
}

func TestSafeCounterAtomic(t *testing.T) {
	got := SafeCounterAtomic(1000)
	if got != 1000 {
		t.Fatalf("got %d want %d", got, 1000)
	}
}
