package billing

import (
	"context"
	"database/sql"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type sBilling struct{}

func init() { service.RegisterBilling(New()) }

func New() *sBilling { return &sBilling{} }

// billingAccount mirrors the accounts table columns including version for optimistic locking.
type billingAccount struct {
	ID           int64     `json:"id"`
	OwnerType    string    `json:"owner_type"`
	OwnerID      int64     `json:"owner_id"`
	Asset        string    `json:"asset"`
	BalanceMicro int64     `json:"balance_micro"`
	Version      int64     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
}

func (a *billingAccount) toDTO() *dto.AccountInfo {
	return &dto.AccountInfo{
		ID:           a.ID,
		OwnerType:    a.OwnerType,
		OwnerID:      a.OwnerID,
		Asset:        a.Asset,
		BalanceMicro: a.BalanceMicro,
		CreatedAt:    a.CreatedAt,
	}
}

type transactionRow struct {
	ID        int64     `json:"id"`
	TxType    string    `json:"tx_type"`
	RefType   string    `json:"ref_type"`
	RefID     int64     `json:"ref_id"`
	CreatedAt time.Time `json:"created_at"`
}

// EnsureAccount finds an account by (owner_type, owner_id, asset) or creates one.
func (s *sBilling) EnsureAccount(ctx context.Context, ownerType string, ownerID int64, asset string) (*dto.AccountInfo, error) {
	acc, err := s.queryAccount(ctx, ownerType, ownerID, asset)
	if err != nil {
		return nil, err
	}
	if acc != nil {
		return acc.toDTO(), nil
	}

	_, err = g.DB().Model("accounts").Ctx(ctx).Data(g.Map{
		"owner_type": ownerType,
		"owner_id":   ownerID,
		"asset":      asset,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create account failed")
	}

	acc, err = s.queryAccount(ctx, ownerType, ownerID, asset)
	if err != nil {
		return nil, err
	}
	if acc == nil {
		return nil, gerror.New("failed to retrieve created account")
	}
	return acc.toDTO(), nil
}

func (s *sBilling) queryAccount(ctx context.Context, ownerType string, ownerID int64, asset string) (*billingAccount, error) {
	var acc billingAccount
	err := g.DB().Model("accounts").Ctx(ctx).
		Where("owner_type", ownerType).
		Where("owner_id", ownerID).
		Where("asset", asset).
		Scan(&acc)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, gerror.Wrap(err, "query account failed")
	}
	if acc.ID == 0 {
		return nil, nil
	}
	return &acc, nil
}

// GetBalance returns the account balance, creating the account if it does not exist.
func (s *sBilling) GetBalance(ctx context.Context, ownerType string, ownerID int64, asset string) (*dto.AccountInfo, error) {
	return s.EnsureAccount(ctx, ownerType, ownerID, asset)
}

// ListTransactions returns paginated transactions for an owner, ordered by newest first.
func (s *sBilling) ListTransactions(ctx context.Context, ownerType string, ownerID int64, page, pageSize int) ([]*dto.TransactionInfo, int, error) {
	// Get account IDs for this owner
	var accounts []*billingAccount
	err := g.DB().Model("accounts").Ctx(ctx).
		Where("owner_type = ?", ownerType).
		Where("owner_id = ?", ownerID).
		Scan(&accounts)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query accounts failed")
	}
	if len(accounts) == 0 {
		return nil, 0, nil
	}

	accountIDs := make([]int64, len(accounts))
	for i, a := range accounts {
		accountIDs[i] = a.ID
	}

	// Count total distinct transactions for this owner
	total, err := g.DB().Model("transactions", "t").
		Ctx(ctx).
		LeftJoin("transaction_entries", "t.id = transaction_entries.tx_id").
		Where("transaction_entries.account_id IN (?)", accountIDs).
		Group("t.id").
		Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count transactions failed")
	}
	if total == 0 {
		return nil, 0, nil
	}

	// Fetch paginated transactions
	offset := (page - 1) * pageSize
	var txs []*transactionRow
	err = g.DB().Model("transactions", "t").
		Ctx(ctx).
		LeftJoin("transaction_entries", "t.id = transaction_entries.tx_id").
		Fields("t.*").
		Where("transaction_entries.account_id IN (?)", accountIDs).
		Group("t.id, t.tx_type, t.ref_type, t.ref_id, t.created_at").
		Order("t.id DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&txs)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query transactions failed")
	}

	// Batch fetch entries for all returned transactions
	txIDs := make([]int64, len(txs))
	for i, tx := range txs {
		txIDs[i] = tx.ID
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
		Where("tx_id IN (?)", txIDs).
		Order("id ASC").
		Scan(&entries)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query entries failed")
	}

	// Build entry lookup map
	entriesByTx := make(map[int64][]dto.TransactionEntryInfo, len(txs))
	for _, e := range entries {
		entriesByTx[e.TxID] = append(entriesByTx[e.TxID], dto.TransactionEntryInfo{
			ID:                e.ID,
			AccountID:         e.AccountID,
			DeltaMicro:        e.DeltaMicro,
			BalanceAfterMicro: e.BalanceAfterMicro,
		})
	}

	result := make([]*dto.TransactionInfo, len(txs))
	for i, tx := range txs {
		result[i] = &dto.TransactionInfo{
			ID:        tx.ID,
			TxType:    tx.TxType,
			RefType:   tx.RefType,
			RefID:     tx.RefID,
			Entries:   entriesByTx[tx.ID],
			CreatedAt: tx.CreatedAt,
		}
	}

	return result, total, nil
}

// CreditAccount adds funds to an account inside a DB transaction.
func (s *sBilling) CreditAccount(ctx context.Context, in dto.CreditAccountIn) (*dto.TransactionInfo, error) {
	var txInfo *dto.TransactionInfo

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Ensure account exists (use direct DB query within transaction)
		acc, err := s.queryAccount(ctx, in.OwnerType, in.OwnerID, in.Asset)
		if err != nil {
			return err
		}
		if acc == nil {
			// Insert account inside the transaction
			r, err := tx.Model("accounts").Data(g.Map{
				"owner_type": in.OwnerType,
				"owner_id":   in.OwnerID,
				"asset":      in.Asset,
			}).Insert()
			if err != nil {
				return gerror.Wrap(err, "create account failed")
			}
			accID, _ := r.LastInsertId()
			acc = &billingAccount{
				ID:        accID,
				OwnerType: in.OwnerType,
				OwnerID:   in.OwnerID,
				Asset:     in.Asset,
				Version:   0,
			}
		}

		// Create transaction record
		txData := g.Map{
			"tx_type": "credit",
		}
		if in.RefType != "" {
			txData["ref_type"] = in.RefType
		}
		if in.RefID != 0 {
			txData["ref_id"] = in.RefID
		}
		txResult, err := tx.Model("transactions").Data(txData).Insert()
		if err != nil {
			return gerror.Wrap(err, "create transaction failed")
		}
		txID, _ := txResult.LastInsertId()

		// Calculate new balance
		newBalance := acc.BalanceMicro + in.AmountMicro

		// Update account with optimistic locking
		rows, err := tx.Model("accounts").
			Where("id = ? AND version = ?", acc.ID, acc.Version).
			Data(g.Map{
				"balance_micro": newBalance,
				"version":       gdb.Raw("version + 1"),
			}).
			Update()
		if err != nil {
			return gerror.Wrap(err, "update account failed")
		}
		affected, _ := rows.RowsAffected()
		if affected == 0 {
			return gerror.New("account version conflict, retry")
		}

		// Create transaction entry
		entryResult, err := tx.Model("transaction_entries").Data(g.Map{
			"tx_id":               txID,
			"account_id":          acc.ID,
			"delta_micro":         in.AmountMicro,
			"balance_after_micro": newBalance,
		}).Insert()
		if err != nil {
			return gerror.Wrap(err, "create transaction entry failed")
		}
		entryID, _ := entryResult.LastInsertId()

		txInfo = &dto.TransactionInfo{
			ID:      txID,
			TxType:  "credit",
			RefType: in.RefType,
			RefID:   in.RefID,
			Entries: []dto.TransactionEntryInfo{
				{
					ID:                entryID,
					AccountID:         acc.ID,
					DeltaMicro:        in.AmountMicro,
					BalanceAfterMicro: newBalance,
				},
			},
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return txInfo, nil
}

// DebitAccount deducts funds from an account inside a DB transaction.
func (s *sBilling) DebitAccount(ctx context.Context, in dto.DebitAccountIn) (*dto.TransactionInfo, error) {
	var txInfo *dto.TransactionInfo

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Ensure account exists (use direct DB query within transaction)
		acc, err := s.queryAccount(ctx, in.OwnerType, in.OwnerID, in.Asset)
		if err != nil {
			return err
		}
		if acc == nil {
			// Insert account inside the transaction
			r, err := tx.Model("accounts").Data(g.Map{
				"owner_type": in.OwnerType,
				"owner_id":   in.OwnerID,
				"asset":      in.Asset,
			}).Insert()
			if err != nil {
				return gerror.Wrap(err, "create account failed")
			}
			accID, _ := r.LastInsertId()
			acc = &billingAccount{
				ID:        accID,
				OwnerType: in.OwnerType,
				OwnerID:   in.OwnerID,
				Asset:     in.Asset,
				Version:   0,
			}
		}

		// Check sufficient balance
		if acc.BalanceMicro < in.AmountMicro {
			return gerror.New("insufficient balance")
		}

		// Create transaction record
		txData := g.Map{
			"tx_type": "debit",
		}
		if in.RefType != "" {
			txData["ref_type"] = in.RefType
		}
		if in.RefID != 0 {
			txData["ref_id"] = in.RefID
		}
		txResult, err := tx.Model("transactions").Data(txData).Insert()
		if err != nil {
			return gerror.Wrap(err, "create transaction failed")
		}
		txID, _ := txResult.LastInsertId()

		// Calculate new balance (negative delta)
		newBalance := acc.BalanceMicro - in.AmountMicro

		// Update account with optimistic locking
		rows, err := tx.Model("accounts").
			Where("id = ? AND version = ?", acc.ID, acc.Version).
			Data(g.Map{
				"balance_micro": newBalance,
				"version":       gdb.Raw("version + 1"),
			}).
			Update()
		if err != nil {
			return gerror.Wrap(err, "update account failed")
		}
		affected, _ := rows.RowsAffected()
		if affected == 0 {
			return gerror.New("account version conflict, retry")
		}

		// Create transaction entry (negative delta for debit)
		entryResult, err := tx.Model("transaction_entries").Data(g.Map{
			"tx_id":               txID,
			"account_id":          acc.ID,
			"delta_micro":         -in.AmountMicro,
			"balance_after_micro": newBalance,
		}).Insert()
		if err != nil {
			return gerror.Wrap(err, "create transaction entry failed")
		}
		entryID, _ := entryResult.LastInsertId()

		txInfo = &dto.TransactionInfo{
			ID:      txID,
			TxType:  "debit",
			RefType: in.RefType,
			RefID:   in.RefID,
			Entries: []dto.TransactionEntryInfo{
				{
					ID:                entryID,
					AccountID:         acc.ID,
					DeltaMicro:        -in.AmountMicro,
					BalanceAfterMicro: newBalance,
				},
			},
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return txInfo, nil
}

// RechargeCredits adds credits to a user's account via the settlement ledger
// and restores any api_keys that were disabled due to overdraft.
func (s *sBilling) RechargeCredits(ctx context.Context, userID, amountCredits int64, refType string, refID int64) (*dto.TransactionInfo, error) {
	if amountCredits <= 0 {
		return nil, gerror.New("amount_credits must be positive")
	}
	tx, err := service.Settlement().CreateRecharge(ctx, dto.RechargeSettlementIn{
		UserID:          userID,
		RefType:         refType,
		RefID:           refID,
		RechargeCredits: amountCredits,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "recharge settlement failed")
	}
	if restoreErr := service.Settlement().RestoreOverdraftKeys(ctx, userID); restoreErr != nil {
		// Don't fail the recharge if key restore fails — log and move on.
		g.Log().Errorf(ctx, "restore overdraft keys failed for user %d after recharge: %v", userID, restoreErr)
	}
	return tx, nil
}

// ========== Admin ==========

func (s *sBilling) ListAllAccounts(ctx context.Context, ownerType, asset string, page, pageSize int) ([]*dto.AccountInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	model := g.DB().Model("accounts").Ctx(ctx)
	if ownerType != "" {
		model = model.Where("owner_type", ownerType)
	}
	if asset != "" {
		model = model.Where("asset", asset)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count accounts failed")
	}

	var accounts []*billingAccount
	offset := (page - 1) * pageSize
	err = model.Order("id DESC").Limit(pageSize).Offset(offset).Scan(&accounts)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query accounts failed")
	}

	list := make([]*dto.AccountInfo, len(accounts))
	for i, a := range accounts {
		list[i] = a.toDTO()
	}
	return list, total, nil
}

func (s *sBilling) ListAllTransactions(ctx context.Context, page, pageSize int) ([]*dto.TransactionInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	total, err := g.DB().Model("transactions").Ctx(ctx).Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count transactions failed")
	}

	var txs []*transactionRow
	offset := (page - 1) * pageSize
	err = g.DB().Model("transactions").Ctx(ctx).
		Order("id DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&txs)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query transactions failed")
	}

	txIDs := make([]int64, len(txs))
	for i, tx := range txs {
		txIDs[i] = tx.ID
	}

	type entryRow struct {
		ID                int64 `json:"id"`
		TxID              int64 `json:"tx_id"`
		AccountID         int64 `json:"account_id"`
		DeltaMicro        int64 `json:"delta_micro"`
		BalanceAfterMicro int64 `json:"balance_after_micro"`
	}

	var entries []*entryRow
	if len(txIDs) > 0 {
		err = g.DB().Model("transaction_entries").Ctx(ctx).
			Where("tx_id IN (?)", txIDs).
			Order("id ASC").
			Scan(&entries)
		if err != nil {
			return nil, 0, gerror.Wrap(err, "query entries failed")
		}
	}

	entriesByTx := make(map[int64][]dto.TransactionEntryInfo, len(txs))
	for _, e := range entries {
		entriesByTx[e.TxID] = append(entriesByTx[e.TxID], dto.TransactionEntryInfo{
			ID:                e.ID,
			AccountID:         e.AccountID,
			DeltaMicro:        e.DeltaMicro,
			BalanceAfterMicro: e.BalanceAfterMicro,
		})
	}

	result := make([]*dto.TransactionInfo, len(txs))
	for i, tx := range txs {
		result[i] = &dto.TransactionInfo{
			ID:        tx.ID,
			TxType:    tx.TxType,
			RefType:   tx.RefType,
			RefID:     tx.RefID,
			Entries:   entriesByTx[tx.ID],
			CreatedAt: tx.CreatedAt,
		}
	}

	return result, total, nil
}
