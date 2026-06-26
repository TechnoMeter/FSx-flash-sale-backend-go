package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/metrics"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/worker"
	"github.com/redis/go-redis/v9"
)

type Stats struct {
	RedisStock        int64  `json:"redisStock"`
	PGOrderCount      int64  `json:"pgOrderCount"`
	StreamLength      int64  `json:"streamLength"`
	PendingCount      int64  `json:"pendingCount"`
	WorkerProcessed   int64  `json:"workerProcessed"`
	ReconcilerLastRun string `json:"reconcilerLastRun"`
	Health            struct {
		Postgres bool `json:"postgres"`
		Redis    bool `json:"redis"`
	} `json:"health"`
}

func StatsHandler(pg *db.PostgresDB, rdb *db.RedisDB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		stats := Stats{}

		stock, err := rdb.Client.Get(ctx, "inventory:product:1").Int64()
		if err != nil && err != redis.Nil {
			stats.Health.Redis = false
		} else {
			stats.Health.Redis = true
			stats.RedisStock = stock
			metrics.InventoryStock.Set(float64(stock)) // update gauge
		}

		count, err := pg.CountOrdersForProduct(ctx, 1)
		if err != nil {
			stats.Health.Postgres = false
		} else {
			stats.Health.Postgres = true
			stats.PGOrderCount = int64(count)
		}

		streamLen, err := rdb.Client.XLen(ctx, "sales:orders").Result()
		if err == nil {
			stats.StreamLength = streamLen
		}

		pending, err := rdb.Client.XPending(ctx, "sales:orders", "flash-sale-workers").Result()
		if err == nil {
			stats.PendingCount = pending.Count
		}

		stats.WorkerProcessed = worker.ProcessedCount()
		stats.ReconcilerLastRun = "N/A" // placeholder

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}