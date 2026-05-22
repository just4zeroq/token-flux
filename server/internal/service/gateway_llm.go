package service

type ILLMRuntime interface{}

var localLLMRuntime ILLMRuntime

func RegisterLLMRuntime(i ILLMRuntime) { localLLMRuntime = i }

func LLMRuntime() ILLMRuntime {
	if localLLMRuntime == nil {
		panic("service.LLMRuntime not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localLLMRuntime
}
