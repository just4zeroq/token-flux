// cmd/api is the main HTTP entry-point. It starts the api ghttp.Server (:8080)
// for business REST API and the gateway ghttp.Server (:8081) for LLM/MCP/Agent
// runtime proxying. Both servers share the same process, same DB pool, same
// in-memory service registry.
package main

import (
	"os"

	_ "ai-platform/internal/logic"

	"ai-platform/internal/boot"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	server := os.Getenv("AI_SERVER")
	if server == "" {
		server = "all"
	}
	boot.RunAPI(server)
}
