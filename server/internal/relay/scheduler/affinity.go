package scheduler

import (
	"sync"
	"time"
)

const (
	AffinityTTL = 1800 // seconds
)

type affinityKey struct {
	UserID    int64
	ModelName string
}

// AffinityStore provides in-memory user+model → channel stickiness.
// Prevents channel thrashing between requests for the same user+model.
type AffinityStore struct {
	mu      sync.RWMutex
	entries map[affinityKey]*affinityEntry
}

type affinityEntry struct {
	ChannelID int64
	HitCount  int
	ExpiresAt time.Time
}

func NewAffinityStore() *AffinityStore {
	return &AffinityStore{
		entries: make(map[affinityKey]*affinityEntry),
	}
}

func (s *AffinityStore) Get(userID int64, modelName string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := affinityKey{UserID: userID, ModelName: modelName}
	entry, ok := s.entries[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return 0, false
	}
	return entry.ChannelID, true
}

func (s *AffinityStore) Set(userID int64, modelName string, channelID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := affinityKey{UserID: userID, ModelName: modelName}
	if existing, ok := s.entries[key]; ok {
		existing.ChannelID = channelID
		existing.HitCount++
		expiry := time.Now().Add(AffinityTTL * time.Second)
		if existing.ExpiresAt.After(expiry) {
			// Keep the later expiry on refresh
			expiry = existing.ExpiresAt
		}
		existing.ExpiresAt = expiry
	} else {
		s.entries[key] = &affinityEntry{
			ChannelID: channelID,
			HitCount:  1,
			ExpiresAt: time.Now().Add(AffinityTTL * time.Second),
		}
	}
}

func (s *AffinityStore) Delete(userID int64, modelName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, affinityKey{UserID: userID, ModelName: modelName})
}

func (s *AffinityStore) DeleteByChannel(channelID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.entries {
		if entry.ChannelID == channelID {
			delete(s.entries, key)
		}
	}
}

func (s *AffinityStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Global affinity instance.
var GlobalAffinity = NewAffinityStore()
