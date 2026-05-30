package router

import (
	"context"
	"sync"

	"ai-platform/cmd/node/internal/provider"
	"ai-platform/cmd/node/internal/types"
)

type Router struct {
	mu    sync.RWMutex
	keys  map[string][]string // model_code -> []key_hash (ordered by priority)
}

func New() *Router {
	return &Router{keys: make(map[string][]string)}
}

// Register adds a key_hash to the pool for a given model.
func (r *Router) Register(modelCode, keyHash string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[modelCode] = append(r.keys[modelCode], keyHash)
}

// Unregister removes a key_hash for a model.
func (r *Router) Unregister(modelCode, keyHash string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.keys[modelCode]
	for i, h := range list {
		if h == keyHash {
			r.keys[modelCode] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// Pick returns the first key_hash for a model (round-robin can be added later).
func (r *Router) Pick(modelCode string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.keys[modelCode]
	if len(list) == 0 {
		return "", &NoKeyError{Model: modelCode}
	}
	// Simple round-robin: move first to end
	first := list[0]
	r.mu.RUnlock()
	r.mu.Lock()
	r.keys[modelCode] = append(list[1:], first)
	r.mu.Unlock()
	r.mu.RLock()
	return first, nil
}

type NoKeyError struct{ Model string }

func (e *NoKeyError) Error() string { return "no key registered for model " + e.Model }

// Route handles a request by picking a key and dispatching to the provider.
func (r *Router) Route(ctx context.Context, env *types.RequestEnvelope) (*types.ChatResponse, error) {
	kh, err := r.Pick(env.Model)
	if err != nil {
		return nil, err
	}
	return provider.Execute(ctx, env, kh)
}

// RouteStream handles a streaming request.
func (r *Router) RouteStream(ctx context.Context, env *types.RequestEnvelope) (<-chan *types.ChunkData, <-chan error) {
	kh, err := r.Pick(env.Model)
	if err != nil {
		errs := make(chan error, 1)
		errs <- err
		close(errs)
		return nil, errs
	}
	return provider.ExecuteStream(ctx, env, kh)
}
