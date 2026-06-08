package executor

import (
	"sync"

	"ai-platform/pkg/model"
)

var (
	registry   = make(map[model.ProviderType]Executor)
	protoReg   = make(map[model.ProtocolType]Executor)
	regMu      sync.RWMutex
)

// Register associates a ProviderType with an Executor implementation.
// Called from init() in each executor package.
func Register(pt model.ProviderType, exec Executor) {
	regMu.Lock()
	registry[pt] = exec
	regMu.Unlock()
}

// RegisterProtocol associates a ProtocolType with an Executor implementation.
func RegisterProtocol(pt model.ProtocolType, exec Executor) {
	regMu.Lock()
	protoReg[pt] = exec
	regMu.Unlock()
}

// GetByProvider returns the Executor for the given ProviderType.
// Falls back to protocol-based lookup, then to openai-compatible default.
func GetByProvider(pt model.ProviderType) Executor {
	regMu.RLock()
	exec, ok := registry[pt]
	regMu.RUnlock()
	if ok {
		return exec
	}
	// Fallback: resolve to protocol
	return GetByProtocol(model.ResolveProtocol(pt))
}

// GetByProtocol returns the Executor for the given ProtocolType.
// Falls back to openai executor for unknown protocols.
func GetByProtocol(pt model.ProtocolType) Executor {
	regMu.RLock()
	exec, ok := protoReg[pt]
	regMu.RUnlock()
	if ok {
		return exec
	}
	// Ultimate fallback: openai (handles most providers)
	regMu.RLock()
	exec, ok = protoReg[model.ProtocolOpenAI]
	regMu.RUnlock()
	if ok {
		return exec
	}
	return nil
}

// All returns all registered executors (for testing/monitoring).
func All() map[model.ProviderType]Executor {
	regMu.RLock()
	defer regMu.RUnlock()
	m := make(map[model.ProviderType]Executor, len(registry))
	for k, v := range registry {
		m[k] = v
	}
	return m
}

// HasProvider returns true if a direct ProviderType mapping exists.
func HasProvider(pt model.ProviderType) bool {
	regMu.RLock()
	_, ok := registry[pt]
	regMu.RUnlock()
	return ok
}
