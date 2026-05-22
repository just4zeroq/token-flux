// cmd/recorder will batch-aggregate usage_records (hourly/daily roll-ups, archival).
// Placeholder: real aggregator lands in P8.
package main

import (
	_ "ai-platform/internal/logic"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
)

func main() {
	ctx := gctx.New()
	g.Log().Info(ctx, "ai-platform-recorder: placeholder, no aggregation started yet")
	g.Wait()
}
