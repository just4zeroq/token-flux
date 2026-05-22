package service

type IAgentRuntime interface{}

var localAgentRuntime IAgentRuntime

func RegisterAgentRuntime(i IAgentRuntime) { localAgentRuntime = i }

func AgentRuntime() IAgentRuntime {
	if localAgentRuntime == nil {
		panic("service.AgentRuntime not registered: missing import _ \"ai-platform/internal/logic\"")
	}
	return localAgentRuntime
}
