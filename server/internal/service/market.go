package service

type IMarket interface{}

var localMarket IMarket

func RegisterMarket(i IMarket) { localMarket = i }

func Market() IMarket {
	if localMarket == nil {
		panic("service.Market not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localMarket
}
