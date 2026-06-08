package router

import (
	"context"
	"sync"

	"ai-platform-node/pkg/provider"
	"ai-platform-node/pkg/types"
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

// Pick returns one key_hash using round-robin.
func (r *Router) Pick(modelCode string) (string, error) {
	r.mu.RLock()
	list := r.keys[modelCode]
	r.mu.RUnlock()
	if len(list) == 0 {
		return "", &NoKeyError{Model: modelCode}
	}
	r.mu.Lock()
	first := list[0]
	r.keys[modelCode] = append(list[1:], first)
	r.mu.Unlock()
	return first, nil
}

// PickAll returns all key hashes for a model, ordered by registration.
func (r *Router) PickAll(modelCode string) ([]string, error) {
	r.mu.RLock()
	list := r.keys[modelCode]
	r.mu.RUnlock()
	if len(list) == 0 {
		return nil, &NoKeyError{Model: modelCode}
	}
	out := make([]string, len(list))
	copy(out, list)
	return out, nil
}

// MarkFailed moves a failed key hash to the end of the list for fallback.
func (r *Router) MarkFailed(modelCode, keyHash string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.keys[modelCode]
	for i, h := range list {
		if h == keyHash {
			r.keys[modelCode] = append(append(list[:i], list[i+1:]...), h)
			return
		}
	}
}

// KeyCount returns the number of key hashes registered for a model.
func (r *Router) KeyCount(modelCode string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.keys[modelCode])
}

type NoKeyError struct{ Model string }

func (e *NoKeyError) Error() string { return "no key registered for model " + e.Model }

// Models returns the list of registered model codes.
func (r *Router) Models() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.keys))
	for code := range r.keys {
		out = append(out, code)
	}
	return out
}

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
