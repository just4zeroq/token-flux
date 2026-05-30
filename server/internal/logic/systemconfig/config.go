// Package systemconfig provides helpers to read tunable parameters from the
// system_configs table. Configs are read on demand (no caching) to guarantee
// freshness — the table is small enough that query overhead is negligible.
//
// Usage:
//
//	creditsPerCNY := systemconfig.GetInt(ctx, "billing.credits_per_cny", 10)
package systemconfig

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// GetInt returns the int64 value of system_configs[key], or defaultVal if the
// key is missing, empty, or non-numeric.
func GetInt(ctx context.Context, key string, defaultVal int64) int64 {
	val, err := g.DB().Model("system_configs").Ctx(ctx).Where("key", key).Value("value")
	if err != nil {
		g.Log().Warningf(ctx, "[SystemConfig] read %q failed: %v — using default %d", key, err, defaultVal)
		return defaultVal
	}
	s := val.String()
	if s == "" {
		return defaultVal
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		g.Log().Warningf(ctx, "[SystemConfig] %q = %q is not int64 — using default %d", key, s, defaultVal)
		return defaultVal
	}
	return n
}

// GetFloat returns the float64 value of system_configs[key], or defaultVal if
// the key is missing, empty, or non-numeric.
func GetFloat(ctx context.Context, key string, defaultVal float64) float64 {
	val, err := g.DB().Model("system_configs").Ctx(ctx).Where("key", key).Value("value")
	if err != nil {
		g.Log().Warningf(ctx, "[SystemConfig] read %q failed: %v — using default %f", key, err, defaultVal)
		return defaultVal
	}
	s := val.String()
	if s == "" {
		return defaultVal
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		g.Log().Warningf(ctx, "[SystemConfig] %q = %q is not float64 — using default %f", key, s, defaultVal)
		return defaultVal
	}
	return n
}

// EnsureConfigs inserts config keys with default values if they don't already
// exist. Called during boot to seed fresh databases without a migration.
func EnsureConfigs(ctx context.Context, configs map[string]string) error {
	for key, value := range configs {
		_, err := g.DB().Model("system_configs").Ctx(ctx).
			Where("key", key).
			Data(g.Map{"value": value}).
			Update()
		if err != nil {
			return gerror.Wrapf(err, "upsert system_config %q", key)
		}
		// If no row was updated, insert.
		rows, _ := g.DB().Model("system_configs").Ctx(ctx).
			Where("key", key).
			Count()
		if rows == 0 {
			_, err = g.DB().Model("system_configs").Ctx(ctx).
				Data(g.Map{"key": key, "value": value}).
				Insert()
			if err != nil {
				return gerror.Wrapf(err, "insert system_config %q", key)
			}
		}
	}
	return nil
}
