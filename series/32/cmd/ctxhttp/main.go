package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

var reqSeq int64

func main() {
	handler := buildHandler(700 * time.Millisecond)

	fmt.Println("=== context + http demo ===")
	simulate(handler, "fast", "/fast", 0)
	simulate(handler, "slow (client 300ms)", "/slow", 300*time.Millisecond)
	simulate(handler, "slow (server 700ms)", "/slow", 0)
}

func buildHandler(timeout time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/fast", handleWork(80*time.Millisecond))
	mux.HandleFunc("/slow", handleWork(1200*time.Millisecond))

	return chain(mux,
		recoverMiddleware,
		requestIDMiddleware,
		timeoutMiddleware(timeout),
		logMiddleware,
		jsonMiddleware,
	)
}

func handleWork(delay time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		if err := work(r.Context(), delay); err != nil {
			status := http.StatusGatewayTimeout
			if errors.Is(err, context.Canceled) {
				status = http.StatusRequestTimeout
			}
			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"request_id": requestIDFromContext(r.Context()),
			"delay":      delay.String(),
			"message":    "ok",
		})
	}
}

func work(ctx context.Context, delay time.Duration) error {
	select {
	case <-time.After(delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func chain(h http.Handler, m ...func(http.Handler) http.Handler) http.Handler {
	wrapped := h
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}
	return wrapped
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := atomic.AddInt64(&reqSeq, 1)
		reqID := fmt.Sprintf("req-%04d", id)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s rid=%s cost=%s\n", r.Method, r.URL.Path, requestIDFromContext(r.Context()), time.Since(start))
	})
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "unknown"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func simulate(handler http.Handler, label, path string, clientTimeout time.Duration) {
	ctx := context.Background()
	if clientTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, clientTimeout)
		defer cancel()
	}

	req := httptest.NewRequest(http.MethodGet, "http://api.local"+path, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rec, req)
	cost := time.Since(start)
	body := strings.TrimSpace(rec.Body.String())
	fmt.Printf("-> %s status=%d cost=%s body=%s\n", label, rec.Code, cost, body)
}
