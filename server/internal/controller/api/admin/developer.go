package admin

import (
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// GET /api/admin/developers — admin management endpoint for developer records.
func ListDevelopers(r *ghttp.Request) {
	list, err := service.Developer().ListActive(r.Context())
	if err != nil {
		fail(r, err.Error())
		return
	}
	ok(r, map[string]any{
		"list": list,
	})
}
