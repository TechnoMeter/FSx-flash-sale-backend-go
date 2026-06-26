package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/db"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/metrics"
	"github.com/TechnoMeter/FSx-flash-sale-backend-go/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	consumerGroup = "flash-sale-workers"
	consumerName  = "worker-1"
	stream        = "sales:orders"
	dlqStream     = "orders:dead"
	maxRetries    = 3
	batchSize     = 50
	pollInterval  = 1 * time.Second
)

var processedOrders atomic.Int64

// ProcessedCount returns the total number of orders successfully persisted.
func ProcessedCount() int64 {
	return processedOrders.Load()
}

type Consumer struct {
	redis  *db.RedisDB
	pg     *db.PostgresDB
	stopCh chan struct{}
}

func NewConsumer(redis *db.RedisDB, pg *db.PostgresDB) *Consumer {
	return &Consumer{
		redis:  redis,
		pg:     pg,
		stopCh: make(chan struct{}),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	if err := c.redis.Client.XGroupCreateMkStream(ctx, stream, consumerGroup, "$").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		slog.Error("XGROUP CREATE failed", "error", err)
		return
	}
	slog.Info("Worker started, listening for orders")

	ticker := time.NewTicker(pollInterval)
	metricsTicker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Worker stopping due to context cancellation")
			c.stopCh <- struct{}{}
			return
		case <-ticker.C:
			c.processBatch(ctx)
		case <-metricsTicker.C:
			c.updateQueueMetrics(ctx)
		}
	}
}

func (c *Consumer) processBatch(ctx context.Context) {
	results, err := c.redis.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{stream, ">"},
		Count:    batchSize,
		Block:    0,
	}).Result()

	if err == redis.Nil || len(results) == 0 {
		return
	}
	if err != nil {
		slog.Error("XREADGROUP failed", "error", err)
		return
	}

	for _, streamMsg := range results {
		for _, msg := range streamMsg.Messages {
			c.handleMessage(ctx, msg)
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, msg redis.XMessage) {
	payload, ok := msg.Values["order"].(string)
	if !ok {
		slog.Warn("invalid message format, acking to avoid poison", "id", msg.ID)
		c.ack(msg.ID)
		return
	}
	var order models.Order
	if err := json.Unmarshal([]byte(payload), &order); err != nil {
		slog.Warn("invalid order JSON, acking", "id", msg.ID, "error", err)
		c.ack(msg.ID)
		return
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := c.pg.InsertOrder(ctx, order)
		if err == nil {
			c.ack(msg.ID)
			processedOrders.Add(1)
			metrics.WorkerProcessedTotal.Inc()
			slog.Info("order persisted", "order_id", order.ID, "product", order.ProductID)
			return
		}
		slog.Warn("INSERT failed, retrying", "attempt", attempt, "error", err)
		time.Sleep(time.Duration(attempt*100) * time.Millisecond)
	}

	c.sendToDLQ(msg, order)
	c.ack(msg.ID)
	slog.Error("order moved to DLQ", "order_id", order.ID)
}

func (c *Consumer) ack(msgID string) {
	if err := c.redis.Client.XAck(context.Background(), stream, consumerGroup, msgID).Err(); err != nil {
		slog.Error("XACK failed", "message_id", msgID, "error", err)
	}
}

func (c *Consumer) sendToDLQ(msg redis.XMessage, order models.Order) {
	payload, _ := json.Marshal(order)
	_, err := c.redis.Client.XAdd(context.Background(), &redis.XAddArgs{
		Stream: dlqStream,
		Values: map[string]interface{}{
			"original_id": msg.ID,
			"order":       string(payload),
			"reason":      "max retries exceeded",
		},
	}).Result()
	if err != nil {
		slog.Error("failed to send to DLQ", "error", err)
	}
}

func (c *Consumer) updateQueueMetrics(ctx context.Context) {
	if length, err := c.redis.Client.XLen(ctx, stream).Result(); err == nil {
		metrics.StreamQueueLength.Set(float64(length))
	}
	if pending, err := c.redis.Client.XPending(ctx, stream, consumerGroup).Result(); err == nil {
		metrics.PendingMessages.Set(float64(pending.Count))
	}
}

func (c *Consumer) WaitStop() <-chan struct{} {
	return c.stopCh
}