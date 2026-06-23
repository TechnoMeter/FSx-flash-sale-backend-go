package reconciler

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/robfig/cron/v3"
    "github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
)

type Reconciler struct {
    redis *db.RedisDB
    pg    *db.PostgresDB
    cron  *cron.Cron
}

func NewReconciler(redis *db.RedisDB, pg *db.PostgresDB) *Reconciler {
    return &Reconciler{
        redis: redis,
        pg:    pg,
        cron:  cron.New(cron.WithSeconds()),
    }
}

func (r *Reconciler) Start(ctx context.Context, schedule string) {
    _, err := r.cron.AddFunc(schedule, func() {
        r.run(ctx)
    })
    if err != nil {
        slog.Error("reconciler schedule error", "error", err)
        return
    }
    r.cron.Start()
    slog.Info("Reconciler started", "schedule", schedule)
}

func (r *Reconciler) run(ctx context.Context) {
    const productID = 1
    stock, err := r.redis.Client.Get(ctx, fmt.Sprintf("inventory:product:%d", productID)).Int64()
    if err != nil {
        slog.Error("reconciler: failed to get redis stock", "error", err)
        return
    }
    count, err := r.pg.CountOrdersForProduct(ctx, productID)
    if err != nil {
        slog.Error("reconciler: failed to get order count", "error", err)
        return
    }
    expected := int64(100 - count)
    if expected < 0 {
        // This indicates a serious inconsistency; log and cap at 0
        slog.Warn("reconciler: expected stock negative, capping to 0",
            "expected", expected, "orders", count)
        expected = 0
    }
    if stock != expected {
        slog.Warn("inventory mismatch detected", "redis_stock", stock, "expected", expected, "orders", count)
        if err := r.redis.Client.Set(ctx, fmt.Sprintf("inventory:product:%d", productID), expected, 0).Err(); err != nil {
            slog.Error("reconciler: failed to update redis", "error", err)
        } else {
            slog.Info("inventory corrected", "new_stock", expected)
        }
    }
}

func (r *Reconciler) Stop() {
    r.cron.Stop()
}