// Package logic is the aggregator: blank-importing this package triggers init()
// in every domain logic subpackage, which in turn registers each domain's
// concrete implementation with the service interface layer.
//
// Always import as:  _ "ai-platform/internal/logic"
package logic

import (
	_ "ai-platform/internal/logic/admin"
	_ "ai-platform/internal/logic/billing"
	_ "ai-platform/internal/logic/catalog"
	_ "ai-platform/internal/logic/gateway"
	_ "ai-platform/internal/logic/identity"
	_ "ai-platform/internal/logic/market"
	_ "ai-platform/internal/logic/pricing"
	_ "ai-platform/internal/logic/settlement"
	_ "ai-platform/internal/logic/usage"
	_ "ai-platform/internal/logic/wallet"
)
