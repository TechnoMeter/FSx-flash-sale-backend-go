package handler

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "time"

    "github.com/google/uuid"
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
    stream   string
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
    ctx := r.Context()
    var req ReserveRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    if req.ProductID != 1 || req.UserID == "" {
        http.Error(w, "invalid product or user", http.StatusBadRequest)
        return
    }

    orderID := uuid.New()
    order := models.Order{
        ID:        orderID,
        ProductID: req.ProductID,
        UserID:    req.UserID,
    }
    payload, _ := json.Marshal(order)

    // Atomic: decrement + XADD in one Lua script
    result, err := h.cb.Execute(func() (any, error) {
        // Call the new script: keys = [inventory_key], args = [stream_key, order_json]
        return h.redis.AtomicReserve.Run(ctx, h.redis.Client,
            []string{fmt.Sprintf("inventory:product:%d", req.ProductID)},
            h.stream, string(payload),
        ).Result()
    })

    if err != nil {
        h.logError(ctx, "atomic reserve failed", err)
        http.Error(w, "inventory service temporarily unavailable", http.StatusServiceUnavailable)
        return
    }

    // The script returns an array: [stock, msg_id]
    arr, ok := result.([]interface{})
    if !ok || len(arr) != 2 {
        h.logError(ctx, "unexpected script return", fmt.Errorf("%v", result))
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    stock, _ := arr[0].(int64)
    //msgID, _ := arr[1].(string)

    switch stock {
    case -2:
        // Sold out – script already rolled back
        writeJSON(w, http.StatusTooManyRequests, ReserveResponse{Success: false, Message: "sold out"})
        return
    case -1:
        // XADD failed – script rolled back
        h.logError(ctx, "XADD failed, rolled back", nil)
        http.Error(w, "failed to place order", http.StatusInternalServerError)
        return
    default:
        // Success: stock is the new inventory count, msgID is the stream message ID
        writeJSON(w, http.StatusOK, ReserveResponse{Success: true, Stock: stock})
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