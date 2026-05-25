package settlement

import (
	"context"
	"database/sql"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type sSettlement struct{}

func init() { service.RegisterSettlement(New()) }

func New() *sSettlement { return &sSettlement{} }

// settlementRecord mirrors the settlement_records table row.
type settlementRecord struct {
	ID                     int64      `json:"id"`
	ProductType            string     `json:"product_type"`
	RefType                string     `json:"ref_type"`
	RefID                  int64      `json:"ref_id"`
	ConsumerUserID         int64      `json:"consumer_user_id"`
	ProviderUserID         int64      `json:"provider_user_id"`
	CostCredits            int64      `json:"cost_credits"`
	ProviderRevenueCredits int64      `json:"provider_revenue_credits"`
	CommissionCredits      int64      `json:"commission_credits"`
	PointsToConsumer       int64      `json:"points_to_consumer"`
	PointsToProvider       int64      `json:"points_to_provider"`
	TransactionID          int64      `json:"transaction_id"`
	Status                 string     `json:"status"`
	ErrorMessage           string     `json:"error_message"`
	CreatedAt              time.Time  `json:"created_at"`
	SettledAt              *time.Time `json:"settled_at"`
}

func (r *settlementRecord) toDTO() *dto.SettlementRecordInfo {
	info := &dto.SettlementRecordInfo{
		ID:                     r.ID,
		ProductType:            r.ProductType,
		RefType:                r.RefType,
		RefID:                  r.RefID,
		ConsumerUserID:         r.ConsumerUserID,
		ProviderUserID:         r.ProviderUserID,
		CostCredits:            r.CostCredits,
		ProviderRevenueCredits: r.ProviderRevenueCredits,
		CommissionCredits:      r.CommissionCredits,
		PointsToConsumer:       r.PointsToConsumer,
		PointsToProvider:       r.PointsToProvider,
		TransactionID:          r.TransactionID,
		Status:                 r.Status,
		ErrorMessage:           r.ErrorMessage,
		CreatedAt:              r.CreatedAt,
	}
	if r.SettledAt != nil {
		info.SettledAt = *r.SettledAt
	}
	return info
}

// getRecord loads a settlement record by ID.
func (s *sSettlement) getRecord(ctx context.Context, id int64) (*settlementRecord, error) {
	var rec settlementRecord
	err := g.DB().Model("settlement_records").Ctx(ctx).
		Where("id", id).
		Scan(&rec)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, gerror.Wrap(err, "settlement record not found")
		}
		return nil, gerror.Wrap(err, "query settlement record failed")
	}
	if rec.ID == 0 {
		return nil, gerror.New("settlement record not found")
	}
	return &rec, nil
}

// getRecordByRef loads a settlement record by (ref_type, ref_id).
func (s *sSettlement) getRecordByRef(ctx context.Context, refType string, refID int64) (*settlementRecord, error) {
	var rec settlementRecord
	err := g.DB().Model("settlement_records").Ctx(ctx).
		Where("ref_type", refType).
		Where("ref_id", refID).
		Scan(&rec)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, gerror.Wrap(err, "query settlement record by ref failed")
	}
	if rec.ID == 0 {
		return nil, nil
	}
	return &rec, nil
}

func (s *sSettlement) getTransaction(ctx context.Context, txID int64) (*dto.TransactionInfo, error) {
	type txRow struct {
		ID        int64     `json:"id"`
		TxType    string    `json:"tx_type"`
		RefType   string    `json:"ref_type"`
		RefID     int64     `json:"ref_id"`
		CreatedAt time.Time `json:"created_at"`
	}
	var tx txRow
	err := g.DB().Model("transactions").Ctx(ctx).
		Where("id", txID).
		Scan(&tx)
	if err != nil {
		return nil, gerror.Wrap(err, "query transaction failed")
	}
	if tx.ID == 0 {
		return nil, gerror.New("transaction not found")
	}

	type entryRow struct {
		ID                int64 `json:"id"`
		TxID              int64 `json:"tx_id"`
		AccountID         int64 `json:"account_id"`
		DeltaMicro        int64 `json:"delta_micro"`
		BalanceAfterMicro int64 `json:"balance_after_micro"`
	}
	var entries []*entryRow
	err = g.DB().Model("transaction_entries").Ctx(ctx).
		Where("tx_id", txID).
		Order("id ASC").
		Scan(&entries)
	if err != nil {
		return nil, gerror.Wrap(err, "query transaction entries failed")
	}

	entryInfos := make([]dto.TransactionEntryInfo, len(entries))
	for i, e := range entries {
		entryInfos[i] = dto.TransactionEntryInfo{
			ID:                e.ID,
			AccountID:         e.AccountID,
			DeltaMicro:        e.DeltaMicro,
			BalanceAfterMicro: e.BalanceAfterMicro,
		}
	}

	return &dto.TransactionInfo{
		ID:        tx.ID,
		TxType:    tx.TxType,
		RefType:   tx.RefType,
		RefID:     tx.RefID,
		Entries:   entryInfos,
		CreatedAt: tx.CreatedAt,
	}, nil
}

// Submit inserts a settlement record with status='pending'.
// Idempotent: if (ref_type, ref_id) already exists, returns the existing record.
func (s *sSettlement) Submit(ctx context.Context, in dto.SettlementSubmitIn) (*dto.SettlementRecordInfo, error) {
	result, err := g.DB().Model("settlement_records").Ctx(ctx).Data(g.Map{
		"product_type":             in.ProductType,
		"ref_type":                 in.RefType,
		"ref_id":                   in.RefID,
		"consumer_user_id":         in.ConsumerUserID,
		"provider_user_id":         in.ProviderUserID,
		"cost_credits":             in.CostCredits,
		"provider_revenue_credits": in.ProviderRevenueCredits,
		"commission_credits":       in.CommissionCredits,
		"points_to_consumer":       in.PointsToConsumer,
		"points_to_provider":       in.PointsToProvider,
		"status":                   "pending",
	}).Insert()
	if err != nil {
		// UNIQUE(ref_type, ref_id) violation — return existing record.
		existing, lookupErr := s.getRecordByRef(ctx, in.RefType, in.RefID)
		if lookupErr != nil {
			return nil, gerror.Wrap(lookupErr, "idempotent lookup failed")
		}
		if existing != nil {
			return existing.toDTO(), nil
		}
		return nil, gerror.Wrap(err, "insert settlement record failed")
	}

	id, _ := result.LastInsertId()
	rec, err := s.getRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	return rec.toDTO(), nil
}

// Settle posts the double-entry ledger and marks the settlement record as settled.
func (s *sSettlement) Settle(ctx context.Context, recordID int64) (*dto.TransactionInfo, error) {
	rec, err := s.getRecord(ctx, recordID)
	if err != nil {
		return nil, err
	}

	if rec.Status == "settled" {
		return s.getTransaction(ctx, rec.TransactionID)
	}

	// Build ledger entries.
	// credits: -cost + provider_revenue + commission == 0 (balanced double-entry).
	// points: one-sided issuance from virtual platform pool; no zero-sum constraint.
	entries := []entryDelta{
		{OwnerType: ownerTypeUser, OwnerID: rec.ConsumerUserID, Asset: assetCredits, Delta: -rec.CostCredits},
		{OwnerType: ownerTypeUser, OwnerID: rec.ProviderUserID, Asset: assetCredits, Delta: rec.ProviderRevenueCredits},
		{OwnerType: ownerTypePlatform, OwnerID: 0, Asset: assetCredits, Delta: rec.CommissionCredits},
	}
	if rec.PointsToConsumer != 0 {
		entries = append(entries, entryDelta{OwnerType: ownerTypeUser, OwnerID: rec.ConsumerUserID, Asset: assetPoints, Delta: rec.PointsToConsumer})
	}
	if rec.PointsToProvider != 0 {
		entries = append(entries, entryDelta{OwnerType: ownerTypeUser, OwnerID: rec.ProviderUserID, Asset: assetPoints, Delta: rec.PointsToProvider})
	}

	var txInfo *dto.TransactionInfo

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		txID, txEntries, postErr := postLedger(ctx, tx, txTypeSettle, "settlement_record", rec.ID, entries)
		if postErr != nil {
			return postErr
		}

		_, updateErr := tx.Model("settlement_records").
			Where("id", rec.ID).
			Data(g.Map{
				"status":         "settled",
				"settled_at":     gtime.Now(),
				"transaction_id": txID,
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "update settlement record settled failed")
		}

		txInfo = &dto.TransactionInfo{
			ID:      txID,
			TxType:  txTypeSettle,
			RefType: "settlement_record",
			RefID:   rec.ID,
			Entries: txEntries,
		}
		return nil
	})

	if err != nil {
		// Mark record as failed outside the transaction.
		g.DB().Model("settlement_records").Ctx(ctx).
			Where("id", rec.ID).
			Data(g.Map{
				"status":        "failed",
				"error_message": err.Error(),
			}).Update()
		return nil, err
	}

	return txInfo, nil
}

// SubmitAndSettle submits a settlement record and immediately settles it.
func (s *sSettlement) SubmitAndSettle(ctx context.Context, in dto.SettlementSubmitIn) (*dto.TransactionInfo, error) {
	rec, err := s.Submit(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.Settle(ctx, rec.ID)
}

// CreateRecharge creates a recharge settlement record and posts a single credit entry.
// Idempotent on (ref_type, ref_id).
func (s *sSettlement) CreateRecharge(ctx context.Context, in dto.RechargeSettlementIn) (*dto.TransactionInfo, error) {
	// Idempotency check: if record exists, return the existing transaction.
	existing, err := s.getRecordByRef(ctx, in.RefType, in.RefID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.TransactionID != 0 {
			return s.getTransaction(ctx, existing.TransactionID)
		}
		return nil, gerror.New("existing recharge record has no transaction")
	}

	entries := []entryDelta{
		{OwnerType: ownerTypeUser, OwnerID: in.UserID, Asset: assetCredits, Delta: in.RechargeCredits},
	}

	var txInfo *dto.TransactionInfo

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Insert settlement record with status='settled' and product_type='recharge'.
		recResult, insertErr := tx.Model("settlement_records").Data(g.Map{
			"product_type":             "recharge",
			"ref_type":                 in.RefType,
			"ref_id":                   in.RefID,
			"consumer_user_id":         in.UserID,
			"provider_user_id":         0,
			"cost_credits":             0,
			"provider_revenue_credits": 0,
			"commission_credits":       0,
			"points_to_consumer":       0,
			"points_to_provider":       0,
			"status":                   "settled",
			"settled_at":               gtime.Now(),
		}).Insert()
		if insertErr != nil {
			// UNIQUE(ref_type, ref_id) violation — concurrent duplicate.
			existing, lookupErr := s.getRecordByRef(ctx, in.RefType, in.RefID)
			if lookupErr != nil {
				return gerror.Wrap(lookupErr, "idempotent lookup after insert conflict failed")
			}
			if existing != nil && existing.TransactionID != 0 {
				txInfo, lookupErr = s.getTransaction(ctx, existing.TransactionID)
				if lookupErr != nil {
					return lookupErr
				}
				return nil
			}
			return gerror.Wrap(insertErr, "insert recharge settlement record failed")
		}
		recID, _ := recResult.LastInsertId()

		txID, txEntries, postErr := postLedger(ctx, tx, txTypeRecharge, "settlement_record", recID, entries)
		if postErr != nil {
			return postErr
		}

		// Link transaction_id back to the settlement record.
		_, updateErr := tx.Model("settlement_records").
			Where("id", recID).
			Data(g.Map{
				"transaction_id": txID,
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "link transaction to recharge failed")
		}

		txInfo = &dto.TransactionInfo{
			ID:      txID,
			TxType:  txTypeRecharge,
			RefType: "settlement_record",
			RefID:   recID,
			Entries: txEntries,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return txInfo, nil
}

// RestoreOverdraftKeys re-enables api_keys previously disabled with reason 'overdraft'
// if the user's credits balance is now >= 0. No-op if balance is still negative.
func (s *sSettlement) RestoreOverdraftKeys(ctx context.Context, userID int64) error {
	// Check current credits balance.
	type accountRow struct {
		BalanceMicro int64 `json:"balance_micro"`
	}
	var acc accountRow
	err := g.DB().Model("accounts").Ctx(ctx).
		Where("owner_type", ownerTypeUser).
		Where("owner_id", userID).
		Where("asset", assetCredits).
		Scan(&acc)
	if err != nil {
		return gerror.Wrap(err, "query credits account failed")
	}
	// If no account exists or balance is negative, nothing to restore.
	if acc.BalanceMicro < 0 {
		return nil
	}

	// Re-enable overdraft-disabled keys for this user.
	_, err = g.DB().Model("api_keys").Ctx(ctx).
		Where("user_id", userID).
		Where("status", 0).
		Where("disabled_reason", "overdraft").
		WhereNull("deleted_at").
		Data(g.Map{
			"status":          1,
			"disabled_reason": "",
			"updated_at":      gtime.Now(),
		}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "restore overdraft keys failed")
	}

	return nil
}