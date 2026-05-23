package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/grand"
)

type sWallet struct{}

func init() { service.RegisterWallet(New()) }

func New() *sWallet { return &sWallet{} }

func (s *sWallet) CreateDepositAddress(ctx context.Context, userID int64, chain string) (*dto.DepositAddressInfo, error) {
	// Check if address already exists for this user+chain
	n, err := g.DB().Model("deposit_addresses").Ctx(ctx).
		Where("user_id = ? AND chain = ?", userID, chain).
		Count()
	if err != nil {
		return nil, fmt.Errorf("check existing address: %w", err)
	}
	if n > 0 {
		return nil, fmt.Errorf("deposit address already exists for chain %s", chain)
	}

	// Generate deterministic address: "0x" + hex(sha256(userID + chain + timestamp)[:20])
	raw := fmt.Sprintf("%d%s%d%s", userID, chain, time.Now().UnixNano(), grand.S(8))
	hash := sha256.Sum256([]byte(raw))
	address := "0x" + hex.EncodeToString(hash[:20])

	hdPath := fmt.Sprintf("m/44'/60'/0'/0/%d", userID)

	result, err := g.DB().Model("deposit_addresses").Ctx(ctx).Insert(g.Map{
		"user_id":    userID,
		"chain":      chain,
		"address":    address,
		"hd_path":    hdPath,
		"created_at": time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("insert deposit address: %w", err)
	}
	id, _ := result.LastInsertId()

	row, err := g.DB().Model("deposit_addresses").Ctx(ctx).
		Where("id = ?", id).One()
	if err != nil {
		return nil, fmt.Errorf("read back deposit address: %w", err)
	}
	if row.IsEmpty() {
		return nil, fmt.Errorf("deposit address not found after insert")
	}

	return &dto.DepositAddressInfo{
		ID:        row["id"].Int64(),
		UserID:    row["user_id"].Int64(),
		Chain:     row["chain"].String(),
		Address:   row["address"].String(),
		HdPath:    row["hd_path"].String(),
		CreatedAt: row["created_at"].GTime().Time,
	}, nil
}

func (s *sWallet) ListDepositAddresses(ctx context.Context, userID int64) ([]*dto.DepositAddressInfo, error) {
	var rows []gdb.Record
	err := g.DB().Model("deposit_addresses").Ctx(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	res := make([]*dto.DepositAddressInfo, len(rows))
	for i, row := range rows {
		res[i] = &dto.DepositAddressInfo{
			ID:        row["id"].Int64(),
			UserID:    row["user_id"].Int64(),
			Chain:     row["chain"].String(),
			Address:   row["address"].String(),
			HdPath:    row["hd_path"].String(),
			CreatedAt: row["created_at"].GTime().Time,
		}
	}
	return res, nil
}

func (s *sWallet) ListDeposits(ctx context.Context, userID int64) ([]*dto.ChainDepositInfo, error) {
	var rows []gdb.Record
	err := g.DB().Model("chain_deposits").Ctx(ctx).
		Where("user_id = ?", userID).
		Order("observed_at DESC").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	res := make([]*dto.ChainDepositInfo, len(rows))
	for i, row := range rows {
		res[i] = &dto.ChainDepositInfo{
			ID:              row["id"].Int64(),
			UserID:          row["user_id"].Int64(),
			Chain:           row["chain"].String(),
			TxHash:          row["tx_hash"].String(),
			FromAddr:        row["from_addr"].String(),
			ToAddr:          row["to_addr"].String(),
			AmountNative:    row["amount_native"].String(),
			AmountUsdAtTime: row["amount_usd_at_time"].Int64(),
			Status:          row["status"].String(),
			ObservedAt:      row["observed_at"].GTime().Time,
		}
	}
	return res, nil
}

func (s *sWallet) CreateWithdraw(ctx context.Context, userID int64, in dto.CreateWithdrawIn) (*dto.WithdrawRequestInfo, error) {
	result, err := g.DB().Model("withdraw_requests").Ctx(ctx).Insert(g.Map{
		"user_id":        userID,
		"chain":          in.Chain,
		"to_address":     in.ToAddress,
		"amount_balance": in.AmountBalance,
		"fee":            0,
		"status":         "pending",
		"created_at":     time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("insert withdraw request: %w", err)
	}
	id, _ := result.LastInsertId()

	row, err := g.DB().Model("withdraw_requests").Ctx(ctx).
		Where("id = ?", id).One()
	if err != nil {
		return nil, fmt.Errorf("read back withdraw request: %w", err)
	}
	if row.IsEmpty() {
		return nil, fmt.Errorf("withdraw request not found after insert")
	}

	return &dto.WithdrawRequestInfo{
		ID:            row["id"].Int64(),
		UserID:        row["user_id"].Int64(),
		Chain:         row["chain"].String(),
		ToAddress:     row["to_address"].String(),
		AmountBalance: row["amount_balance"].Int64(),
		Fee:           row["fee"].Int64(),
		Status:        row["status"].String(),
		CreatedAt:     row["created_at"].GTime().Time,
	}, nil
}

func (s *sWallet) ListWithdrawals(ctx context.Context, userID int64) ([]*dto.WithdrawRequestInfo, error) {
	var rows []gdb.Record
	err := g.DB().Model("withdraw_requests").Ctx(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Scan(&rows)
	if err != nil {
		return nil, err
	}
	res := make([]*dto.WithdrawRequestInfo, len(rows))
	for i, row := range rows {
		res[i] = &dto.WithdrawRequestInfo{
			ID:            row["id"].Int64(),
			UserID:        row["user_id"].Int64(),
			Chain:         row["chain"].String(),
			ToAddress:     row["to_address"].String(),
			AmountBalance: row["amount_balance"].Int64(),
			Fee:           row["fee"].Int64(),
			Status:        row["status"].String(),
			CreatedAt:     row["created_at"].GTime().Time,
		}
	}
	return res, nil
}
