// Local-quickstart shim: `go run ./server` boots the API server. The real
// production entry-point is cmd/api, which this file mirrors.
package main

import (
	_ "ai-platform/internal/logic"

	"ai-platform/internal/boot"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() { boot.RunAPI() }
