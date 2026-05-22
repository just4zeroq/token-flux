// cmd/settlement will run the periodic provider revenue settlement cron
// (move provider credits -> provider balance once per day).
// Placeholder: real cron lands in P7.
package main

import (
	_ "ai-platform/internal/logic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	ctx := gctx.New()
	g.Log().Info(ctx, "ai-platform-settlement: placeholder, no cron started yet")
	g.Wait()
}
