// Package handler — async settlement queue with pluggable backend.
//
// Architecture:
//   - SettlementQueue interface abstracts the message broker (Redis, Kafka, etc.).
//   - InitQueue() selects the implementation before StartSettlementWorkers().
//   - EnqueueSettlement pushes tasks via the interface.
//   - Workers pull tasks via the interface's Dequeue().
//   - Each worker: INSERT usage record + settlement record in one DB transaction.
//   - DebitAccount runs outside the transaction (loss accepted; overdraft keys disabled).
//   - Quota updates run synchronously on the request path (see updateQuotaSync in handler.go).
//   - Provider payout delayed 7 days via settlement ticker scanning settlement_records.
//
// To add a new queue backend (e.g. Kafka):
//   1. Create queue_kafka.go with an implementation of SettlementQueue.
//   2. In boot.go, call handler.InitQueue(handler.NewKafkaQueue()) before
//      handler.StartSettlementWorkers(ctx, 4).
//
// Settlement lifecycle:
//   usage occurs → sync quota update (request path) → EnqueueSettlement (via queue)
//   → worker pulls task → INSERT usage + settlement (tx) → DebitAccount (async)
//   7 days later → ticker/ProcessSettlements → provider credited (earnings) + status=settled
package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/relay/common"
	"ai-platform/internal/service"
)

// maxSettlementRetries is the number of times a failed task will be re-enqueued
// before being dropped permanently.
const maxSettlementRetries = 3

// settlementTimeout is the maximum time a worker spends on one task.
const settlementTimeout = 30 * time.Second

// SettlementTask carries everything needed to record one LLM request's billing.
type SettlementTask struct {
	UserID           int64          `json:"user_id"`
	ApiKeyID         int64          `json:"api_key_id"`
	ModelKeyID       int64          `json:"model_key_id"`
	KeyModelID       int64          `json:"key_model_id"`
	ProviderUserID   int64          `json:"provider_user_id"`
	ChannelID        int64          `json:"channel_id"`
	ModelSpecID      int64          `json:"model_spec_id"`
	RequestID        string         `json:"request_id"`
	Capability       string         `json:"capability"`
	IsStream         bool             `json:"is_stream"`
	Usage            *common.Usage    `json:"usage"`
	Cost             int64            `json:"cost"`
	ProviderShareBps int              `json:"provider_share_bps"`
	StartTime        time.Time        `json:"start_time"`
	RetryCount       int              `json:"retry_count"`       // incremented on re-enqueue
	RequestBody      json.RawMessage  `json:"request_body,omitempty"`
	ResponseBody     json.RawMessage  `json:"response_body,omitempty"`
	ResponseText     string           `json:"response_text,omitempty"`       // lazy tokenize 用
	ModelName         string           `json:"model_name,omitempty"`              // tiktoken 模型选择
	OutputPricePer1K   int64          `json:"output_price_per_1k,omitempty"` // worker 算 cost
	CacheHitPricePer1K  int64         `json:"cache_hit_price_per_1k,omitempty"`
	CacheMissPricePer1K int64         `json:"cache_miss_price_per_1k,omitempty"`
}

// SettlementQueue abstracts the message broker for settlement tasks.
// Implementations: Redis (built-in), Kafka (future), etc.
type SettlementQueue interface {
	// Enqueue pushes a task. Must be safe for concurrent callers.
	Enqueue(ctx context.Context, task SettlementTask) error

	// Dequeue blocks until a task is available or ctx is cancelled.
	// Returns:
	//   task, nil  — a task was dequeued
	//   nil, nil   — queue is empty (caller should retry after a brief pause)
	//   nil, error — permanent error or ctx cancelled (caller should stop)
	Dequeue(ctx context.Context) (*SettlementTask, error)

	// Name returns a human-readable name for logging (e.g. "redis", "kafka").
	Name() string
}

const (
	settlementDefaultWorkers  = 4
	settlementShutdownTimeout = 10 * time.Second
	// DefaultSettlementCycleDays is the number of days before a provider can be paid.
	// During this window consumers can dispute charges.
	DefaultSettlementCycleDays = 7
	// settlementScanInterval is how often the ticker checks for expired pending settlements.
	settlementScanInterval = 5 * time.Minute
)

var (
	globalQueue  SettlementQueue
	settleCtx    context.Context
	settleCancel context.CancelFunc
)

// InitQueue sets the global settlement queue implementation.
// Must be called before StartSettlementWorkers, or StartSettlementWorkers
// will default to the Redis backend.
func InitQueue(q SettlementQueue) {
	globalQueue = q
}

// StartSettlementWorkers launches numWorkers goroutines that consume from the
// configured queue plus a ticker for periodic settlement of expired records.
// Call before g.Wait().
func StartSettlementWorkers(parentCtx context.Context, numWorkers int) {
	if numWorkers <= 0 {
		numWorkers = settlementDefaultWorkers
	}

	// Default to Redis if no explicit InitQueue call.
	if globalQueue == nil {
		globalQueue = newRedisQueue()
		if globalQueue == nil {
			return // newRedisQueue already logged Fatalf
		}
	}

	settleCtx, settleCancel = context.WithCancel(parentCtx)

	for i := 0; i < numWorkers; i++ {
		go settlementWorker(i)
	}
	go settlementTicker()

	g.Log().Infof(parentCtx, "[Settlement] started %d workers + ticker (queue=%s), cycle=%dd",
		numWorkers, globalQueue.Name(), DefaultSettlementCycleDays)
}

// StopSettlementWorkers runs a final settlement pass, then signals workers to stop.
func StopSettlementWorkers() {
	if settleCancel == nil {
		return
	}

	// Final settlement pass before shutdown.
	ctx := context.Background()
	if n, err := ProcessSettlements(ctx); err != nil {
		g.Log().Errorf(ctx, "[Settlement] final settlement pass failed: %v", err)
	} else if n > 0 {
		g.Log().Infof(ctx, "[Settlement] final settlement pass: %d records settled", n)
	}

	settleCancel()
	g.Log().Info(ctx, "[Settlement] workers signaled to stop")
}

// EnqueueSettlement pushes a settlement task to the configured queue.
func EnqueueSettlement(task SettlementTask) {
	if globalQueue == nil {
		g.Log().Error(context.Background(), "[Settlement] queue not initialized, dropping task")
		return
	}
	if err := globalQueue.Enqueue(context.Background(), task); err != nil {
		g.Log().Errorf(context.Background(),
			"[Settlement] enqueue failed: queue=%s user=%d cost=%d err=%v",
			globalQueue.Name(), task.UserID, task.Cost, err)
	}
}

// ProcessSettlements scans for pending settlement records whose settle_at has expired
// and credits the provider's earnings account using the pre-computed revenue.
// Returns the number of records processed.
// Can be called by admin API for manual settlement, or by the periodic ticker.
func ProcessSettlements(ctx context.Context) (int, error) {
	var rows []struct {
		ID                     int64 `json:"id"`
		ProviderUserID         int64 `json:"provider_user_id"`
		ProviderRevenueCredits int64 `json:"provider_revenue_credits"`
	}

	err := g.DB().Model("settlement_records").Ctx(ctx).
		Where("status", "pending").
		Where("settle_at <= ?", time.Now()).
		Fields("id, provider_user_id, provider_revenue_credits").
		Scan(&rows)
	if err != nil {
		return 0, gerror.Wrap(err, "query pending settlements failed")
	}

	settled := 0
	for _, row := range rows {
		// Credit provider's earnings account (revenue pre-computed at usage time).
		if row.ProviderRevenueCredits > 0 {
			if _, err := service.Billing().CreditAccount(ctx, dto.CreditAccountIn{
				Asset:       "earnings",
				AmountMicro: row.ProviderRevenueCredits,
				OwnerType:   "user",
				OwnerID:     row.ProviderUserID,
				RefType:     "settlement",
				RefID:       row.ID,
			}); err != nil {
				g.Log().Errorf(ctx, "[Settlement] credit provider earnings failed: settlement=%d provider=%d amount=%d err=%v",
					row.ID, row.ProviderUserID, row.ProviderRevenueCredits, err)
				continue
			}
		}

		// Mark as settled.
		if _, err := g.DB().Model("settlement_records").Ctx(ctx).
			Where("id", row.ID).
			Data(g.Map{
				"status":     "settled",
				"settled_at": time.Now(),
			}).Update(); err != nil {
			g.Log().Errorf(ctx, "[Settlement] mark settled failed: id=%d err=%v", row.ID, err)
			continue
		}

		settled++
	}

	if settled > 0 {
		g.Log().Infof(ctx, "[Settlement] processed %d settlements (pending→settled)", settled)
	}
	return settled, nil
}

// calcRevenue computes the provider's share of cost in micro-credits.
// providerShareBps is in basis points (10000 = 100%).
func calcRevenue(cost int64, providerShareBps int) int64 {
	if providerShareBps <= 0 {
		return cost * 8000 / 10000 // fallback: 80%
	}
	return cost * int64(providerShareBps) / 10000
}

// settlementWorker pulls tasks from the configured queue in a blocking loop.
func settlementWorker(id int) {
	g.Log().Infof(context.Background(), "[Settlement] worker-%d started (queue=%s)", id, globalQueue.Name())

	for {
		select {
		case <-settleCtx.Done():
			g.Log().Infof(context.Background(), "[Settlement] worker-%d stopped", id)
			return
		default:
		}

		task, err := globalQueue.Dequeue(settleCtx)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				return
			}
			g.Log().Errorf(context.Background(), "[Settlement] worker-%d dequeue error: %v", id, err)
			time.Sleep(time.Second)
			continue
		}
		if task == nil {
			continue // timeout — no task, loop back for shutdown check
		}

		processSettlement(*task, id)
	}
}

// processSettlement handles one settlement task.
// Quota updated on request path for normal path, or here for lazy tokenize tasks.
func processSettlement(task SettlementTask, workerID int) {
	ctx, cancel := context.WithTimeout(context.Background(), settlementTimeout)
	defer cancel()

	// === 0. Lazy tokenize: upstream didn't return usage, count tokens now. ===
	if (task.Usage == nil || task.Usage.TotalTokens == 0) && task.ResponseText != "" {
		if globalTokenizer == nil {
			g.Log().Errorf(ctx, "[Settlement] worker-%d lazy tokenize: no tokenizer configured, will retry", workerID)
			if task.RetryCount < maxSettlementRetries {
				task.RetryCount++
				EnqueueSettlement(task)
			}
			return
		}
		tokens, err := globalTokenizer.CountTokens(ctx, task.ModelName, task.ResponseText)
		if err != nil {
			g.Log().Errorf(ctx, "[Settlement] worker-%d lazy tokenize failed: %v", workerID, err)
			if task.RetryCount < maxSettlementRetries {
				task.RetryCount++
				EnqueueSettlement(task)
			}
			return
		}
		task.Usage = &common.Usage{
			CompletionTokens: tokens,
			TotalTokens:      tokens,
		}
		task.Cost = task.OutputPricePer1K * int64(tokens) / 1000
		g.Log().Infof(ctx, "[Settlement] worker-%d lazy tokenize: %d tokens, cost=%d", workerID, tokens, task.Cost)
	}

	// Extract usage fields for TX + chat log (both scopes).
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	cacheHitTokens := 0
	cacheMissTokens := int64(0)
	if task.Usage != nil {
		promptTokens = task.Usage.PromptTokens
		completionTokens = task.Usage.CompletionTokens
		totalTokens = task.Usage.TotalTokens
		if task.Usage.PromptTokensDetails != nil {
			cacheHitTokens = task.Usage.PromptTokensDetails.CachedTokens
		}
		cacheMissTokens = int64(task.Usage.PromptTokens - cacheHitTokens)
		if cacheMissTokens < 0 {
			cacheMissTokens = int64(task.Usage.PromptTokens)
			cacheHitTokens = 0
		}
	}

	// -- Transaction: usage + settlement records (atomic) --
	var usageID int64

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		settleAt := time.Now().AddDate(0, 0, DefaultSettlementCycleDays)
		revenue := calcRevenue(task.Cost, task.ProviderShareBps)
		commission := task.Cost - revenue

		// 1. INSERT usage record.

		// 1. INSERT usage record.
		insertRes, err := tx.Model("llm_usage_records").Data(g.Map{
			"consumer_user_id":         task.UserID,
			"virtual_key_id":           task.ApiKeyID,
			"model_spec_id":            task.ModelSpecID,
			"model_key_id":             task.ModelKeyID,
			"key_model_id":             task.KeyModelID,
			"provider_user_id":         task.ProviderUserID,
			"channel_id":               task.ChannelID,
			"request_id":               task.RequestID,
			"capability":               task.Capability,
			"is_stream":                task.IsStream,
			"input_tokens":             promptTokens,
			"cache_hit_tokens":         cacheHitTokens,
			"cache_miss_tokens":        cacheMissTokens,
			"output_tokens":            completionTokens,
			"total_tokens":             totalTokens,
			"cost_credits":             task.Cost,
			"provider_revenue_credits": revenue,
			"commission_credits":       commission,
			"status":                   "success",
		}).Insert()
		if err != nil {
			return gerror.Wrap(err, "insert usage record failed")
		}
		uid, _ := insertRes.LastInsertId()
		usageID = uid

		// 2. INSERT settlement record (pending, settle_at = now + cycle).
		if _, err := tx.Model("settlement_records").Data(g.Map{
			"product_type":             "llm",
			"ref_type":                 "llm_usage_record",
			"ref_id":                   usageID,
			"consumer_user_id":         task.UserID,
			"provider_user_id":         task.ProviderUserID,
			"cost_credits":             task.Cost,
			"provider_revenue_credits": revenue,
			"commission_credits":       commission,
			"status":                   "pending",
			"settle_at":                settleAt,
		}).Insert(); err != nil {
			return gerror.Wrap(err, "insert settlement record failed")
		}

		return nil
	})

	if err != nil {
		g.Log().Errorf(ctx, "[Settlement] worker-%d tx failed: user=%d key_model=%d cost=%d err=%v",
			workerID, task.UserID, task.KeyModelID, task.Cost, err)
		// Re-enqueue for retry (up to maxSettlementRetries).
		if task.RetryCount < maxSettlementRetries {
			task.RetryCount++
			EnqueueSettlement(task)
			g.Log().Infof(ctx, "[Settlement] worker-%d re-enqueued task (retry %d/%d): user=%d",
				workerID, task.RetryCount, maxSettlementRetries, task.UserID)
		} else {
			g.Log().Errorf(ctx, "[Settlement] worker-%d dropping task after %d retries: user=%d cost=%d",
				workerID, maxSettlementRetries, task.UserID, task.Cost)
		}
		return
	}

	// 3. Debit consumer (outside transaction — separate billing tx).
	// For lazy tokenize tasks, quota was NOT updated on request path — skip guard here,
	// worker handles quota sync after debit.
	if task.Cost > 0 {
		if _, err := service.Billing().DebitAccount(ctx, dto.DebitAccountIn{
			Asset:       "credits",
			AmountMicro: task.Cost,
			OwnerType:   "user",
			OwnerID:     task.UserID,
			RefType:     "llm_usage",
			RefID:       usageID,
		}); err != nil {
			g.Log().Errorf(ctx, "[Settlement] worker-%d debit failed: user=%d amount=%d err=%v",
				workerID, task.UserID, task.Cost, err)
			// Overdraft protection: disable user's api keys.
			disableOverdraftKeys(ctx, task.UserID)
		}

		// 3b. Quota update: normal path already did this on request path, lazy task needs it here.
		if task.ResponseText != "" {
			updateQuotaSync(ctx, task.ModelKeyID, task.KeyModelID, task.Cost)
		}
	}

	// 4. Save chat log (best-effort - won't fail settlement).
	if len(task.RequestBody) > 0 {
		if _, err := g.DB().Model("llm_chat_logs").Ctx(ctx).Data(g.Map{
			"user_id":           task.UserID,
			"api_key_id":        task.ApiKeyID,
			"model_spec_id":     task.ModelSpecID,
			"request_id":        task.RequestID,
			"capability":        task.Capability,
			"is_stream":         task.IsStream,
			"request_body":      string(task.RequestBody),
			"response_body":     func() string { if len(task.ResponseBody) > 0 { return string(task.ResponseBody) }; return "{}" }(),
			"prompt_tokens":     func() int { if task.Usage != nil { return task.Usage.PromptTokens }; return 0 }(),
			"completion_tokens": func() int { if task.Usage != nil { return task.Usage.CompletionTokens }; return 0 }(),
			"total_tokens":      func() int { if task.Usage != nil { return task.Usage.TotalTokens }; return 0 }(),
			"cost_credits":      task.Cost,
		}).Insert(); err != nil {
			g.Log().Errorf(ctx, "[ChatLog] insert failed: request=%s err=%v", task.RequestID, err)
		}
	}
}// disableOverdraftKeys disables all active api_keys for a user due to overdraft.
func disableOverdraftKeys(ctx context.Context, userID int64) {
	res, err := g.DB().Model("api_keys").Ctx(ctx).
		Where("user_id", userID).
		Where("status", 1).
		Data(g.Map{
			"status":          0,
			"disabled_reason": "overdraft",
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[Settlement] disable keys failed for user=%d: %v", userID, err)
		return
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		g.Log().Warningf(ctx, "[Settlement] disabled %d api_keys for user=%d (overdraft)", n, userID)
	}
}

// settlementTicker periodically scans for expired pending settlements and processes them.
func settlementTicker() {
	ticker := time.NewTicker(settlementScanInterval)
	defer ticker.Stop()

	g.Log().Infof(context.Background(), "[Settlement] ticker started (interval=%v)", settlementScanInterval)

	for {
		select {
		case <-ticker.C:
			n, err := ProcessSettlements(context.Background())
			if err != nil {
				g.Log().Errorf(context.Background(), "[Settlement] ticker settlement failed: %v", err)
			} else if n > 0 {
				g.Log().Infof(context.Background(), "[Settlement] ticker: %d records settled", n)
			}
		case <-settleCtx.Done():
			g.Log().Info(context.Background(), "[Settlement] ticker stopped")
			return
		}
	}
}
