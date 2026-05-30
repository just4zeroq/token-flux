// Package handler — Redis implementation of SettlementQueue.
package handler

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	redisSettlementQueueKey  = "settlement:queue"
	redisSettlementRetryKey  = "settlement:retry"
	redisBPopTimeout         = 3 // seconds for BRPOP timeout
)

// redisQueue implements SettlementQueue using Redis List (LPUSH / BRPOP).
// Tasks are serialized as JSON and persisted in Redis — survives server restart.
type redisQueue struct {
	queueKey  string
	bpTimeout int64
}

// newRedisQueue creates a Redis-backed queue and verifies connectivity.
// Returns nil (after logging Fatalf) if Redis is unreachable.
func newRedisQueue() *redisQueue {
	ctx := context.Background()
	if _, err := g.Redis().Do(ctx, "PING"); err != nil {
		g.Log().Fatalf(ctx, "[Settlement] Redis not available: %v", err)
		return nil
	}
	return &redisQueue{
		queueKey:  redisSettlementQueueKey,
		bpTimeout: redisBPopTimeout,
	}
}

func (q *redisQueue) Enqueue(ctx context.Context, task SettlementTask) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = g.Redis().Do(ctx, "LPUSH", q.queueKey, string(data))
	return err
}

func (q *redisQueue) Dequeue(ctx context.Context) (*SettlementTask, error) {
	result, err := g.Redis().Do(ctx, "BRPOP", q.queueKey, q.bpTimeout)
	if err != nil {
		return nil, err
	}
	if result.IsNil() {
		// Timeout — no task available.
		return nil, nil
	}

	arr := result.Array()
	if len(arr) < 2 {
		return nil, nil
	}
	taskJSON, ok := arr[1].(string)
	if !ok {
		return nil, nil
	}

	var task SettlementTask
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (q *redisQueue) Name() string {
	return "redis"
}
