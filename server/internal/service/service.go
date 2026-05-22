// Package service defines the cross-module domain interfaces. Each domain
// (identity, catalog, market, wallet, billing, pricing, usage, and the three
// gateway runtimes) exposes a single interface plus a package-level accessor.
//
// Domain implementations live under internal/logic/<domain>/ and register
// themselves via init() calls to RegisterXxx. This keeps logic packages
// referenced only by their concrete interface, eliminating import cycles
// between domains.
//
// Usage:
//
//	// In a controller or another logic package:
//	user, err := service.Identity().GetUser(ctx, userID)
package service
