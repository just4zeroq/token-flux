package service

type IUsage interface{}

var localUsage IUsage

func RegisterUsage(i IUsage) { localUsage = i }

func Usage() IUsage {
	if localUsage == nil {
		panic("service.Usage not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localUsage
}
