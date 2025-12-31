package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

type snapshot struct {
	heapAlloc      uint64
	heapSys        uint64
	heapObjects    uint64
	numGC          uint32
	pauseTotalNs   uint64
	lastPauseNs    uint64
	nextGC         uint64
	gccpuFraction  float64
	lastGCTimeUnix int64
}

func main() {
	fmt.Println("=== GC 何时运行 & STW 影响演示 ===")
	fmt.Println("可选：开启 gctrace 观察更细节：GODEBUG=gctrace=1 go run ./series/15/cmd/gcplay")

	phase("默认 GOGC=100", func() {
		debug.SetGCPercent(100)
		runChurn(80_000, 256, true)
	})

	phase("更激进 GOGC=20（更频繁 GC，堆更小）", func() {
		debug.SetGCPercent(20)
		runChurn(80_000, 256, true)
	})

	phase("更宽松 GOGC=200（更少 GC，堆更大）", func() {
		debug.SetGCPercent(200)
		runChurn(80_000, 256, true)
	})

	phase("禁用 GC（仅用于演示）", func() {
		debug.SetGCPercent(-1)
		runChurn(120_000, 512, false)
	})

	phase("手动触发 runtime.GC()", func() {
		debug.SetGCPercent(100)
		runtime.GC()
	})
}

func phase(name string, fn func()) {
	fmt.Printf("\n--- %s ---\n", name)
	before := readSnapshot()
	printSnapshot("before", before)

	start := time.Now()
	fn()
	cost := time.Since(start)

	after := readSnapshot()
	printSnapshot("after ", after)
	fmt.Printf("time=%s | gc+%d | pause+%s\n",
		cost,
		int(after.numGC-before.numGC),
		time.Duration(after.pauseTotalNs-before.pauseTotalNs))
}

func runChurn(objects int, size int, keep bool) {
	var keepAlive [][]byte
	if keep {
		keepAlive = make([][]byte, 0, objects/8)
	}

	for i := 0; i < objects; i++ {
		b := make([]byte, size)
		b[0] = byte(i)
		if keep && i%8 == 0 {
			keepAlive = append(keepAlive, b)
		}
	}

	// Make sure allocations are not optimized away.
	runtime.KeepAlive(keepAlive)
}

func readSnapshot() snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	lastPause := uint64(0)
	if m.NumGC > 0 {
		lastPause = m.PauseNs[(m.NumGC+255)%256]
	}

	lastGCTime := int64(0)
	if m.LastGC != 0 {
		lastGCTime = int64(m.LastGC / 1e9)
	}

	return snapshot{
		heapAlloc:      m.HeapAlloc,
		heapSys:        m.HeapSys,
		heapObjects:    m.HeapObjects,
		numGC:          m.NumGC,
		pauseTotalNs:   m.PauseTotalNs,
		lastPauseNs:    lastPause,
		nextGC:         m.NextGC,
		gccpuFraction:  m.GCCPUFraction,
		lastGCTimeUnix: lastGCTime,
	}
}

func printSnapshot(label string, s snapshot) {
	fmt.Printf("%s | heap_alloc=%s heap_sys=%s heap_obj=%d next_gc=%s num_gc=%d last_pause=%s gccpu=%.4f last_gc=%d\n",
		label,
		bytes(s.heapAlloc),
		bytes(s.heapSys),
		s.heapObjects,
		bytes(s.nextGC),
		s.numGC,
		time.Duration(s.lastPauseNs),
		s.gccpuFraction,
		s.lastGCTimeUnix,
	)
}

func bytes(n uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
	)
	switch {
	case n >= MB:
		return fmt.Sprintf("%.2fMB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.2fKB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%dB", n)
	}
}
