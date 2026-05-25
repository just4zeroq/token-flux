// Package gateway aggregates the three runtime subpackages (llm, mcp, agent).
// Blank-importing this package triggers init() in each subpackage which in
// turn registers itself with the service layer.
package gateway

import (
	_ "ai-platform/internal/logic/gateway/agent"
	_ "ai-platform/internal/logic/gateway/mcp"
)
