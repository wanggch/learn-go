package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func main() {
	base := "http://mock.local"

	fastClient := newClient(800 * time.Millisecond)
	slowClient := newClient(2 * time.Second)

	fmt.Println("base:", base)

	doGet("fast /fast", fastClient, base+"/fast", 0)
	doGet("fast /slow", fastClient, base+"/slow", 0)
	doGet("slow /slow", slowClient, base+"/slow", 0)
	doGet("slow /slow (ctx 500ms)", slowClient, base+"/slow", 500*time.Millisecond)

	fastClient.CloseIdleConnections()
	slowClient.CloseIdleConnections()
}

func newClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: mockTransport{},
	}
}

type mockTransport struct{}

func (mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	delay := 80 * time.Millisecond
	body := `{"ok":true,"path":"fast"}`
	if strings.Contains(req.URL.Path, "/slow") {
		delay = 1200 * time.Millisecond
		body = `{"ok":true,"path":"slow"}`
	}

	select {
	case <-time.After(delay):
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")

	return resp, nil
}

func (mockTransport) CloseIdleConnections() {}

func doGet(label string, client *http.Client, url string, ctxTimeout time.Duration) {
	start := time.Now()

	ctx := context.Background()
	if ctxTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ctxTimeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fmt.Printf("%s: build request error=%v\n", label, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%s: error=%v cost=%s\n", label, err, time.Since(start))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		fmt.Printf("%s: read error=%v cost=%s\n", label, err, time.Since(start))
		return
	}

	fmt.Printf("%s: status=%d cost=%s body=%s\n", label, resp.StatusCode, time.Since(start), string(body))
}
