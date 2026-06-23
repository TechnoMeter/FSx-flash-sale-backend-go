package db

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/models"
)

type PostgresDB struct {
    Pool *pgxpool.Pool
}

func NewPostgres(dsn string) (*PostgresDB, error) {
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, fmt.Errorf("parse DSN: %w", err)
    }
    // Connection pool tuning
    config.MaxConns = 20
    config.MinConns = 5
    config.MaxConnLifetime = 5 * time.Minute

    pool, err := pgxpool.NewWithConfig(context.Background(), config)
    if err != nil {
        return nil, fmt.Errorf("connect to postgres: %w", err)
    }
    if err := pool.Ping(context.Background()); err != nil {
        return nil, fmt.Errorf("ping postgres: %w", err)
    }
    slog.Info("PostgreSQL connected")
    return &PostgresDB{Pool: pool}, nil
}

func (p *PostgresDB) InsertOrder(ctx context.Context, order models.Order) error {
    query := `INSERT INTO orders (id, product_id, user_id) VALUES ($1, $2, $3)`
    _, err := p.Pool.Exec(ctx, query, order.ID, order.ProductID, order.UserID)
    return err
}

func (p *PostgresDB) CountOrdersForProduct(ctx context.Context, productID int) (int, error) {
    var count int
    err := p.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE product_id=$1", productID).Scan(&count)
    return count, err
}