// Local-quickstart shim: `go run ./server` boots the API server. The real
// production entry-point is cmd/api, which this file mirrors.
package main

import (
	"os"

	_ "ai-platform/internal/logic"

	"ai-platform/internal/boot"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
)

func main() {
	server := os.Getenv("AI_SERVER")
	if server == "" {
		server = "all"
	}
	boot.RunAPI(server)
}
