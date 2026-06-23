package handler

import (
    "net/http"
    "fmt"

    "github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "OK")
}

func ReadinessCheck(pg *db.PostgresDB, rdb *db.RedisDB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        // Check Postgres
        if err := pg.Pool.Ping(ctx); err != nil {
            http.Error(w, "postgres not ready", http.StatusServiceUnavailable)
            return
        }
        // Check Redis
        if err := rdb.Client.Ping(ctx).Err(); err != nil {
            http.Error(w, "redis not ready", http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "ready")
    }
}

// Simple metrics endpoint (replace with Prometheus if you add it)
func Metrics(w http.ResponseWriter, r *http.Request) {
    // For demo, just return a plain text with some basic stats
    // In real production, you'd use promhttp.Handler()
    w.Header().Set("Content-Type", "text/plain")
    fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests\n")
    fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
    fmt.Fprintf(w, "http_requests_total %d\n", 0) // placeholder
}