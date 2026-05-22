package catalog

import "ai-platform/internal/service"

type sCatalog struct{}

func init() { service.RegisterCatalog(New()) }

func New() *sCatalog { return &sCatalog{} }
