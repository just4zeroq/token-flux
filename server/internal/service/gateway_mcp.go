package service

type IMCPRuntime interface{}

var localMCPRuntime IMCPRuntime

func RegisterMCPRuntime(i IMCPRuntime) { localMCPRuntime = i }

func MCPRuntime() IMCPRuntime {
	if localMCPRuntime == nil {
		panic("service.MCPRuntime not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localMCPRuntime
}
