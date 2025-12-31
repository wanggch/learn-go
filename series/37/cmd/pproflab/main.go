package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"time"
)

func main() {
	outDir := profileDir()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	cpuPath := filepath.Join(outDir, "cpu.pprof")
	memPath := filepath.Join(outDir, "heap.pprof")

	cpuFile, err := os.Create(cpuPath)
	if err != nil {
		panic(err)
	}

	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		_ = cpuFile.Close()
		panic(err)
	}

	result := workload()

	pprof.StopCPUProfile()
	_ = cpuFile.Close()

	runtime.GC()
	memFile, err := os.Create(memPath)
	if err != nil {
		panic(err)
	}
	if err := pprof.WriteHeapProfile(memFile); err != nil {
		_ = memFile.Close()
		panic(err)
	}
	_ = memFile.Close()

	fmt.Println("done")
	fmt.Printf("result checksum: %s\n", result)
	fmt.Printf("cpu profile: %s\n", cpuPath)
	fmt.Printf("heap profile: %s\n", memPath)
	fmt.Println("\nview tips:")
	fmt.Println("go tool pprof -top", cpuPath)
	fmt.Println("go tool pprof -top", memPath)
}

func workload() string {
	start := time.Now()
	data := make([]string, 0, 40000)
	for i := 0; i < 40000; i++ {
		data = append(data, strings.Repeat("go", i%10+1))
	}
	for i := 0; i < 4; i++ {
		sort.Strings(data)
	}

	h := sha256.New()
	for _, item := range data {
		h.Write([]byte(item))
	}
	elapsed := time.Since(start)
	fmt.Printf("workload finished in %s\n", elapsed)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func profileDir() string {
	if _, err := os.Stat(filepath.Join("series", "37")); err == nil {
		return filepath.Join("series", "37", "tmp")
	}
	return filepath.Join("tmp")
}
