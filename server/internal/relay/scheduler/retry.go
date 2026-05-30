package scheduler

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
)

// DefaultMaxRetries is the fallback when system_configs has no value.
const DefaultMaxRetries = 3

// GetMaxRetries reads gateway.max_retries from system_configs (cached in config table).
func GetMaxRetries(ctx context.Context) int {
	val, err := g.DB().Model("system_configs").Ctx(ctx).
		Where("key", "gateway.max_retries").
		Value("value")
	if err != nil || val == nil {
		return DefaultMaxRetries
	}
	n := val.Int()
	if n <= 0 {
		return DefaultMaxRetries
	}
	return n
}

// ShouldRetry determines if a failed upstream call should be retried.
func ShouldRetry(err error, retryCount, maxRetries int) bool {
	if retryCount >= maxRetries {
		return false
	}
	return IsRetryable(err)
}

// IsRetryable returns true for transient network/HTTP errors.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	// Network-level errors — always retry.
	if _, ok := err.(net.Error); ok {
		return true
	}

	// DNS/connect/timeout
	if strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "tls handshake") {
		return true
	}

	// URL errors
	if _, ok := err.(*url.Error); ok {
		return true
	}

	return false
}
