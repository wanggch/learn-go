package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type order struct {
	ID        int    `json:"id"`
	Item      string `json:"item"`
	Price     int    `json:"price"`
	CreatedAt string `json:"created_at"`
}

type createOrderRequest struct {
	Item  string `json:"item"`
	Price int    `json:"price"`
}

type store struct {
	mu     sync.RWMutex
	nextID int
	items  map[int]order
}

func newStore() *store {
	return &store{
		nextID: 1000,
		items:  make(map[int]order),
	}
}

func (s *store) create(req createOrderRequest) order {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	ord := order{
		ID:        s.nextID,
		Item:      req.Item,
		Price:     req.Price,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.items[ord.ID] = ord
	return ord
}

func (s *store) list() []order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]order, 0, len(s.items))
	for _, ord := range s.items {
		result = append(result, ord)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func (s *store) get(id int) (order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ord, ok := s.items[id]
	return ord, ok
}

type api struct {
	store *store
}

func main() {
	handler := buildHandler()

	fmt.Println("=== net/http server demo ===")
	simulate(handler, http.MethodGet, "/health", nil)
	simulate(handler, http.MethodPost, "/orders", createOrderRequest{Item: "latte", Price: 28})
	simulate(handler, http.MethodPost, "/orders", createOrderRequest{Item: "sandwich", Price: 38})
	simulate(handler, http.MethodGet, "/orders", nil)
	simulate(handler, http.MethodGet, "/orders/1001", nil)
	simulate(handler, http.MethodGet, "/orders/4040", nil)
	simulate(handler, http.MethodPut, "/orders/1001", nil)
}

func buildHandler() http.Handler {
	store := newStore()
	api := &api{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.handleHealth)
	mux.HandleFunc("/orders", api.handleOrders)
	mux.HandleFunc("/orders/", api.handleOrder)

	return chain(mux, recoverMiddleware, logMiddleware, jsonMiddleware)
}

func (a *api) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.store.list())
	case http.MethodPost:
		var req createOrderRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Item == "" || req.Price <= 0 {
			writeError(w, http.StatusBadRequest, "item and price are required")
			return
		}
		ord := a.store.create(req)
		writeJSON(w, http.StatusCreated, ord)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *api) handleOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/orders/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	ord, ok := a.store.get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	writeJSON(w, http.StatusOK, ord)
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

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s cost=%s\n", r.Method, r.URL.Path, time.Since(start))
	})
}

func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	limited := io.LimitReader(r.Body, 1<<20)
	dec := json.NewDecoder(limited)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("invalid json: unexpected extra data")
	}
	return nil
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

func simulate(handler http.Handler, method, path string, payload any) {
	var body io.Reader
	if payload != nil {
		data, _ := json.Marshal(payload)
		body = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, "http://api.local"+path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	fmt.Printf("-> %s %s status=%d body=%s\n", method, path, rec.Code, strings.TrimSpace(rec.Body.String()))
}
