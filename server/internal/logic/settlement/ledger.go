package settlement

import (
	"context"
	"database/sql"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"ai-platform/internal/model/dto"
)

// Account and asset constants used in ledger operations.
const (
	ownerTypeUser     = "user"
	ownerTypePlatform = "platform"
	assetCredits      = "credits"
	assetPoints       = "points"

	txTypeSettle   = "settle"
	txTypeRecharge = "recharge"
)

// entryDelta describes one side of a ledger entry.
// Negative delta = debit (decreases balance); positive = credit (increases balance).
type entryDelta struct {
	OwnerType string
	OwnerID   int64  // 0 for platform
	Asset     string
	Delta     int64 // micro units; negative = debit
}

// postLedger inserts a transactions row + N transaction_entries inside tx.
// Returns the new transaction_id and entry info.
//
// For credits: sum of all credit deltas must be zero (balanced double-entry),
// unless txType is txTypeRecharge (single-entry credit from external source).
// Points are treated as one-sided issuance from a virtual platform pool;
// no zero-sum constraint is enforced for points.
//
// Auto-creates accounts (with version=0) if missing.
// Uses optimistic locking on accounts.version when updating balance_micro.
func postLedger(ctx context.Context, tx gdb.TX, txType, refType string, refID int64, entries []entryDelta) (int64, []dto.TransactionEntryInfo, error) {
	if len(entries) == 0 {
		return 0, nil, gerror.New("postLedger: at least one entry required")
	}

	// Validate credits balance (zero-sum) for non-recharge transactions.
	if txType != txTypeRecharge {
		var creditsSum int64
		for _, e := range entries {
			if e.Asset == assetCredits {
				creditsSum += e.Delta
			}
		}
		if creditsSum != 0 {
			return 0, nil, gerror.Newf("credits imbalance: sum=%d, expected 0", creditsSum)
		}
	}

	// Insert transactions row.
	txResult, err := tx.Model("transactions").Data(g.Map{
		"tx_type":  txType,
		"ref_type": refType,
		"ref_id":   refID,
	}).Insert()
	if err != nil {
		return 0, nil, gerror.Wrap(err, "insert transaction failed")
	}
	txID, _ := txResult.LastInsertId()

	entryInfos := make([]dto.TransactionEntryInfo, 0, len(entries))

	for _, e := range entries {
		// Look up existing account within the transaction.
		type accountRow struct {
			ID           int64 `json:"id"`
			BalanceMicro int64 `json:"balance_micro"`
			Version      int64 `json:"version"`
		}
		var acc accountRow
		err := tx.Model("accounts").
			Where("owner_type", e.OwnerType).
			Where("owner_id", e.OwnerID).
			Where("asset", e.Asset).
			Scan(&acc)
		if err != nil && err != sql.ErrNoRows {
			return 0, nil, gerror.Wrap(err, "query account failed")
		}

		// Auto-create account if missing.
		if acc.ID == 0 {
			r, insertErr := tx.Model("accounts").Data(g.Map{
				"owner_type": e.OwnerType,
				"owner_id":   e.OwnerID,
				"asset":      e.Asset,
			}).Insert()
			if insertErr != nil {
				return 0, nil, gerror.Wrap(insertErr, "create account failed")
			}
			accID, _ := r.LastInsertId()
			acc.ID = accID
			acc.BalanceMicro = 0
			acc.Version = 0
		}

		newBalance := acc.BalanceMicro + e.Delta

		// Update account with optimistic locking.
		rows, updateErr := tx.Model("accounts").
			Where("id = ? AND version = ?", acc.ID, acc.Version).
			Data(g.Map{
				"balance_micro": newBalance,
				"version":       gdb.Raw("version + 1"),
			}).
			Update()
		if updateErr != nil {
			return 0, nil, gerror.Wrap(updateErr, "update account balance failed")
		}
		affected, _ := rows.RowsAffected()
		if affected == 0 {
			return 0, nil, gerror.New("account version conflict, retry")
		}

		// Insert transaction entry.
		entryResult, entryErr := tx.Model("transaction_entries").Data(g.Map{
			"tx_id":               txID,
			"account_id":          acc.ID,
			"delta_micro":         e.Delta,
			"balance_after_micro": newBalance,
		}).Insert()
		if entryErr != nil {
			return 0, nil, gerror.Wrap(entryErr, "insert transaction entry failed")
		}
		entryID, _ := entryResult.LastInsertId()

		entryInfos = append(entryInfos, dto.TransactionEntryInfo{
			ID:                entryID,
			AccountID:         acc.ID,
			DeltaMicro:        e.Delta,
			BalanceAfterMicro: newBalance,
		})
	}

	return txID, entryInfos, nil
}