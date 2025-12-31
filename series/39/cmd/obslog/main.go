package main

import (
	"errors"
	"math/rand"
	"os"
	"strconv"
	"time"

	"learn-go/series/39/internal/obs"
)

const (
	kindTimeout  = "timeout"
	kindNotFound = "not_found"
)

func main() {
	logger := obs.NewLogger("billing")

	trace := obs.TraceID()
	start := time.Now()

	rand.Seed(time.Now().UnixNano())
	id := randomID()
	if v := os.Getenv("INVOICE_ID"); v != "" {
		if forced, err := strconv.Atoi(v); err == nil && forced > 0 {
			id = forced
		}
	}
	amount, err := fetchInvoice(id)
	if err != nil {
		logger.ErrorWithTrace(err,
			obs.Str("trace", trace),
			obs.Str("op", "fetch_invoice"),
			obs.Int("invoice_id", id),
			obs.Duration("cost", time.Since(start)),
		)

		switch {
		case errors.Is(err, errTimeout()):
			logger.Info("fallback to cache", obs.Str("trace", trace))
		case obs.IsKind(err, kindNotFound):
			logger.Info("notify billing", obs.Str("trace", trace))
		default:
			logger.Info("alert oncall", obs.Str("trace", trace))
		}
		return
	}

	logger.Info("invoice loaded",
		obs.Str("trace", trace),
		obs.Int("invoice_id", id),
		obs.Int("amount", amount),
		obs.Duration("cost", time.Since(start)),
	)
}

func fetchInvoice(id int) (int, error) {
	trace := obs.TraceID()
	if id%5 == 0 {
		return 0, obs.Wrap("fetchInvoice", kindTimeout, trace, errTimeout())
	}
	if id%7 == 0 {
		return 0, obs.Wrap("fetchInvoice", kindNotFound, trace, errMissing())
	}
	return id * 10, nil
}

func errTimeout() error {
	return errors.New("db timeout")
}

func errMissing() error {
	return errors.New("invoice missing")
}

func randomID() int {
	return rand.Intn(50) + 1
}
