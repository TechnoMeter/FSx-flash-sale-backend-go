package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	"github.com/TechnoMeter/flash-sale-backend-go/internal/db"
	"github.com/TechnoMeter/flash-sale-backend-go/internal/handler"
	"github.com/TechnoMeter/flash-sale-backend-go/internal/reconciler"
	"github.com/TechnoMeter/flash-sale-backend-go/internal/worker"
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
	// Load .env (ignore if not present)
	_ = godotenv.Load()

	// Setup structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Environment variables
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	rateLimitRPS := 10 // default, can read from env

	// Connect to PostgreSQL and Redis
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

	// Seed Redis inventory (only if not exists)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Set inventory to 100 for product 1, but only if key missing (idempotent)
	rdb.Client.SetNX(ctx, "inventory:product:1", 100, 0)

	// Start background worker
	consumer := worker.NewConsumer(rdb, pg)
	workerCtx, workerStop := context.WithCancel(context.Background())
	go consumer.Start(workerCtx)

	// Start reconciler (every 5 min)
	rec := reconciler.NewReconciler(rdb, pg)
	rec.Start(context.Background(), "0 */5 * * * *") // every 5 minutes
	defer rec.Stop()

	// HTTP router
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger) // structured logging by chi's logger (or use slog middleware)

	// Rate limiting per IP (sliding window)
	r.Use(httprate.LimitByIP(rateLimitRPS, time.Second))

	// Health & readiness
	r.Get("/health", handler.HealthCheck)
	r.Get("/ready", handler.ReadinessCheck(pg, rdb))

	// Landing page
	r.Get("/", handler.IndexPage)

	// Metrics (Prometheus) – we'll use a simple /metrics endpoint for demo
	r.Get("/metrics", handler.Metrics)

	// Reserve endpoint
	reserveHandler := handler.NewReserveHandler(rdb, pg)
	r.Post("/reserve", reserveHandler.ServeHTTP)

	// Server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
		// Timeouts to avoid slow clients
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
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

	// Stop worker and reconciler
	workerStop()
	<-consumer.WaitStop()

	// Shutdown HTTP server with timeout
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("Shutdown complete")
}
