package handler

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"          
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/redis/go-redis/v9"  
    "github.com/sony/gobreaker/v2"
    "github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
    "github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/models"
)

type ReserveRequest struct {
    ProductID int    `json:"product_id"`
    UserID    string `json:"user_id"`
}

type ReserveResponse struct {
    Success bool  `json:"success"`
    Stock   int64 `json:"stock,omitempty"`
    Message string `json:"message,omitempty"`
}

type ReserveHandler struct {
    redis    *db.RedisDB
    pg       *db.PostgresDB
    cb       *gobreaker.CircuitBreaker[any]
    stream   string // Redis stream key
}

func NewReserveHandler(redis *db.RedisDB, pg *db.PostgresDB) *ReserveHandler {
    // Circuit breaker: fail after 5 consecutive errors, reset after 30s
    cb := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
        Name:        "redis-eval",
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
    ctx := r.Context()
    var req ReserveRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // Validate input
    if req.ProductID != 1 || req.UserID == "" {
        http.Error(w, "invalid product or user", http.StatusBadRequest)
        return
    }

    // ---------- 1. Atomic decrement via Lua (with circuit breaker) ----------
    stockVal, err := h.cb.Execute(func() (any, error) {
        return h.redis.Decr.Run(ctx, h.redis.Client, []string{fmt.Sprintf("inventory:product:%d", req.ProductID)}).Int64()
    })
    if err != nil {
        // Circuit breaker open or Redis error
        h.logError(ctx, "redis decr failed", err)
        http.Error(w, "inventory service temporarily unavailable", http.StatusServiceUnavailable)
        return
    }
    stock := stockVal.(int64)

    if stock == -2 {
        // Sold out
        writeJSON(w, http.StatusTooManyRequests, ReserveResponse{Success: false, Message: "sold out"})
        return
    }

    // ---------- 2. Generate UUID and push to stream ----------
    orderID := uuid.New()
    order := models.Order{
        ID:        orderID,
        ProductID: req.ProductID,
        UserID:    req.UserID,
    }
    payload, _ := json.Marshal(order)

    // XADD
    xaddErr := h.redis.Client.XAdd(ctx, &redis.XAddArgs{
        Stream: h.stream,
        Values: map[string]interface{}{"order": string(payload)},
    }).Err()

    if xaddErr != nil {
        // Compensating rollback: INCR back the stock
        h.redis.Client.Incr(ctx, fmt.Sprintf("inventory:product:%d", req.ProductID))
        h.logError(ctx, "xadd failed, rolled back", xaddErr)
        http.Error(w, "failed to place order", http.StatusInternalServerError)
        return
    }

    // ---------- 3. Success ----------
    writeJSON(w, http.StatusOK, ReserveResponse{Success: true, Stock: stock})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(data)
}

func (h *ReserveHandler) logError(ctx context.Context, msg string, err error) {
    slog.ErrorContext(ctx, msg, "error", err)
}