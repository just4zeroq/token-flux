package service

type ICatalog interface{}

var localCatalog ICatalog

func RegisterCatalog(i ICatalog) { localCatalog = i }

func Catalog() ICatalog {
	if localCatalog == nil {
		panic("service.Catalog not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localCatalog
}
