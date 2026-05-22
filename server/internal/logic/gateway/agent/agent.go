package agent

import "ai-platform/internal/service"

type sAgent struct{}

func init() { service.RegisterAgentRuntime(New()) }

func New() *sAgent { return &sAgent{} }
