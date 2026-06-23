package db

import (
    "context"
    _ "embed"
    "fmt"
    "log/slog"
    "time"

    "github.com/redis/go-redis/v9"
)

//go:embed decr.lua
var decrLuaScript string

//go:embed atomic_reserve.lua
var atomicReserveScript string

type RedisDB struct {
    Client          *redis.Client
    Decr            *redis.Script
    AtomicReserve   *redis.Script   // new
}

func NewRedis(addr string) (*RedisDB, error) {
    opt, err := redis.ParseURL(addr)
    if err != nil {
        return nil, fmt.Errorf("parse redis URL: %w", err)
    }
    client := redis.NewClient(opt)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("redis ping: %w", err)
    }
    slog.Info("Redis connected")
    return &RedisDB{
        Client:        client,
        Decr:          redis.NewScript(decrLuaScript),
        AtomicReserve: redis.NewScript(atomicReserveScript),
    }, nil
}