package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/handler"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/metrics"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/reconciler"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/worker"
)

func runMigrations(pg *db.PostgresDB) error {
	content, err := os.ReadFile("migrations/001_init.up.sql")
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}
	_, err = pg.Pool.Exec(context.Background(), string(content))
	if err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}
	return nil
}

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	rateLimitRPS := 10
	if rps := os.Getenv("RATE_LIMIT_RPS"); rps != "" {
		if val, err := strconv.Atoi(rps); err == nil {
			rateLimitRPS = val
		}
	}

	pg, err := db.NewPostgres(dbURL)
	if err != nil {
		slog.Error("failed to connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Pool.Close()

	if err := runMigrations(pg); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied successfully")

	rdb, err := db.NewRedis(redisURL)
	if err != nil {
		slog.Error("failed to connect redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Client.Set(ctx, "inventory:product:1", 100, 0).Err(); err != nil {
		slog.Warn("failed to seed inventory", "error", err)
	}

	metrics.RegisterAll()

	consumer := worker.NewConsumer(rdb, pg)
	workerCtx, workerStop := context.WithCancel(context.Background())
	go consumer.Start(workerCtx)

	rec := reconciler.NewReconciler(rdb, pg)
	rec.Start(context.Background(), "0 */5 * * * *")
	defer rec.Stop()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)

	// Custom rate limiter that skips if X-Test-Mode header is present
	rateLimiter := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Test-Mode") == "true" {
				next.ServeHTTP(w, r)
				return
			}
			httprate.LimitByIP(rateLimitRPS, time.Second)(next).ServeHTTP(w, r)
		})
	}
	r.Use(rateLimiter)

	r.Get("/health", handler.HealthCheck)
	r.Get("/ready", handler.ReadinessCheck(pg, rdb))
	r.Get("/", handler.IndexPage)
	r.Get("/reset", handler.ResetStock(rdb))
	r.Get("/stock", handler.StockHandler(rdb))
	r.Get("/stats", handler.StatsHandler(pg, rdb))
	r.Handle("/metrics", promhttp.Handler())

	reserveHandler := handler.NewReserveHandler(rdb, pg)
	r.Post("/reserve", reserveHandler.ServeHTTP)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Shutting down gracefully...")

	workerStop()
	<-consumer.WaitStop()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("Shutdown complete")
}