// cmd/watcher will host on-chain deposit observers for BNB / Base / TON.
// Placeholder: only blank-imports logic so service interfaces are registered;
// real wallet.Watcher.Start lands in P6 (wallet plan).
package main

import (
	_ "ai-platform/internal/logic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	ctx := gctx.New()
	g.Log().Info(ctx, "ai-platform-watcher: placeholder, no chain workers started yet")
	g.Wait()
}
