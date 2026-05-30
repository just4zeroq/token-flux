package service

import (
	"context"

	"ai-platform/internal/model/dto"
)

type IInvoice interface {
	List(ctx context.Context, status string, page, pageSize int) ([]*dto.InvoiceInfo, int, error)
	Get(ctx context.Context, id int64) (*dto.InvoiceInfo, error)
	Create(ctx context.Context, in dto.CreateInvoiceIn) (*dto.InvoiceInfo, error)
	UpdateStatus(ctx context.Context, id int64, in dto.UpdateInvoiceStatusIn) error
}

var localInvoice IInvoice

func RegisterInvoice(i IInvoice) { localInvoice = i }

func Invoice() IInvoice {
	if localInvoice == nil {
		panic("service.Invoice not registered")
	}
	return localInvoice
}
