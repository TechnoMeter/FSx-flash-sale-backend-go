package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/sony/gobreaker/v2"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/metrics"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/models"
)

type ReserveRequest struct {
	ProductID int    `json:"product_id"`
	UserID    string `json:"user_id"`
}

type ReserveResponse struct {
	Success bool   `json:"success"`
	Stock   int64  `json:"stock,omitempty"`
	Message string `json:"message,omitempty"`
}

type ReserveHandler struct {
	redis  *db.RedisDB
	pg     *db.PostgresDB
	cb     *gobreaker.CircuitBreaker[any]
	stream string
}

func NewReserveHandler(redis *db.RedisDB, pg *db.PostgresDB) *ReserveHandler {
	cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name:        "redis-reserve",
		MaxRequests: 3,
		Interval:    0,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	})
	return &ReserveHandler{
		redis:  redis,
		pg:     pg,
		cb:     cb,
		stream: "sales:orders",
	}
}

func (h *ReserveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	path := "/reserve"
	var statusCode int

	defer func() {
		duration := time.Since(start).Seconds()
		metrics.RequestDuration.WithLabelValues(path).Observe(duration)
		// statusCode is set in each branch
	}()

	ctx := r.Context()
	var req ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		statusCode = http.StatusBadRequest
		http.Error(w, "invalid request", statusCode)
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		return
	}

	if req.ProductID != 1 || req.UserID == "" {
		statusCode = http.StatusBadRequest
		http.Error(w, "invalid product or user", statusCode)
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		return
	}

	orderID := uuid.New()
	order := models.Order{
		ID:        orderID,
		ProductID: req.ProductID,
		UserID:    req.UserID,
	}
	payload, _ := json.Marshal(order)

	result, err := h.cb.Execute(func() (any, error) {
		return h.redis.AtomicReserve.Run(ctx, h.redis.Client,
			[]string{fmt.Sprintf("inventory:product:%d", req.ProductID)},
			h.stream, string(payload),
		).Result()
	})

	if err != nil {
		statusCode = http.StatusServiceUnavailable
		h.logError(ctx, "atomic reserve failed", err)
		http.Error(w, "inventory service temporarily unavailable", statusCode)
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		// Circuit breaker state is updated automatically by gobreaker, but we can read it:
		if h.cb.State() == gobreaker.StateOpen {
			metrics.CircuitBreakerState.Set(1)
		} else if h.cb.State() == gobreaker.StateHalfOpen {
			metrics.CircuitBreakerState.Set(2)
		} else {
			metrics.CircuitBreakerState.Set(0)
		}
		return
	}

	arr, ok := result.([]interface{})
	if !ok || len(arr) != 2 {
		statusCode = http.StatusInternalServerError
		h.logError(ctx, "unexpected script return", fmt.Errorf("%v", result))
		http.Error(w, "internal error", statusCode)
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		return
	}

	stock, _ := arr[0].(int64)

	switch stock {
	case -2:
		statusCode = http.StatusTooManyRequests
		writeJSON(w, statusCode, ReserveResponse{Success: false, Message: "sold out"})
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		metrics.InventoryStock.Set(0)
		return
	case -1:
		statusCode = http.StatusInternalServerError
		h.logError(ctx, "XADD failed, rolled back", nil)
		http.Error(w, "failed to place order", statusCode)
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		return
	default:
		statusCode = http.StatusOK
		writeJSON(w, statusCode, ReserveResponse{Success: true, Stock: stock})
		metrics.RequestTotal.WithLabelValues(path, strconv.Itoa(statusCode)).Inc()
		metrics.InventoryStock.Set(float64(stock))
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *ReserveHandler) logError(ctx context.Context, msg string, err error) {
	slog.ErrorContext(ctx, msg, "error", err)
}