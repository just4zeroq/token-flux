package invoice

import (
	"context"
	"fmt"
	"time"

	"ai-platform/internal/model/dto"
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

type sInvoice struct{}

func init() { service.RegisterInvoice(New()) }

func New() *sInvoice { return &sInvoice{} }

type invoiceRow struct {
	ID            int64      `json:"id"`
	UserID        int64      `json:"user_id"`
	AmountCredits int64      `json:"amount_credits"`
	RefType       string     `json:"ref_type"`
	RefID         int64      `json:"ref_id"`
	InvoiceNumber string     `json:"invoice_number"`
	Status        string     `json:"status"`
	Notes         string     `json:"notes"`
	IssuedAt      *time.Time `json:"issued_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (r *invoiceRow) toDTO() *dto.InvoiceInfo {
	return &dto.InvoiceInfo{
		ID:            r.ID,
		UserID:        r.UserID,
		AmountCredits: r.AmountCredits,
		RefType:       r.RefType,
		RefID:         r.RefID,
		InvoiceNumber: r.InvoiceNumber,
		Status:        r.Status,
		Notes:         r.Notes,
		IssuedAt:      r.IssuedAt,
		CreatedAt:     r.CreatedAt,
	}
}

func (s *sInvoice) List(ctx context.Context, status string, page, pageSize int) ([]*dto.InvoiceInfo, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	model := g.DB().Model("invoices").Ctx(ctx)
	if status != "" {
		model = model.Where("status", status)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "count invoices failed")
	}

	var rows []*invoiceRow
	offset := (page - 1) * pageSize
	err = model.Order("id DESC").Limit(pageSize).Offset(offset).Scan(&rows)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "query invoices failed")
	}

	list := make([]*dto.InvoiceInfo, len(rows))
	for i, r := range rows {
		list[i] = r.toDTO()
	}
	return list, total, nil
}

func (s *sInvoice) Get(ctx context.Context, id int64) (*dto.InvoiceInfo, error) {
	var row invoiceRow
	err := g.DB().Model("invoices").Ctx(ctx).Where("id", id).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "query invoice failed")
	}
	if row.ID == 0 {
		return nil, gerror.New("invoice not found")
	}
	return row.toDTO(), nil
}

func (s *sInvoice) Create(ctx context.Context, in dto.CreateInvoiceIn) (*dto.InvoiceInfo, error) {
	number := fmt.Sprintf("INV-%d-%d", time.Now().Unix(), in.UserID%100000)
	r, err := g.DB().Model("invoices").Ctx(ctx).Data(g.Map{
		"user_id":        in.UserID,
		"amount_credits": in.AmountCredits,
		"ref_type":       in.RefType,
		"ref_id":         in.RefID,
		"invoice_number": number,
		"status":         "pending",
		"notes":          in.Notes,
		"created_at":     gtime.Now(),
	}).Insert()
	if err != nil {
		return nil, gerror.Wrap(err, "create invoice failed")
	}

	id, _ := r.LastInsertId()
	return s.Get(ctx, id)
}

func (s *sInvoice) UpdateStatus(ctx context.Context, id int64, in dto.UpdateInvoiceStatusIn) error {
	data := g.Map{
		"status": in.Status,
	}
	if in.Status == "issued" {
		data["issued_at"] = gtime.Now()
	}
	if in.Notes != "" {
		data["notes"] = in.Notes
	}

	rows, err := g.DB().Model("invoices").Ctx(ctx).Where("id", id).Data(data).Update()
	if err != nil {
		return gerror.Wrap(err, "update invoice failed")
	}
	affected, _ := rows.RowsAffected()
	if affected == 0 {
		return gerror.New("invoice not found")
	}
	return nil
}
