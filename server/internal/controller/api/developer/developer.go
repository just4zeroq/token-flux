package developer

import (
	"ai-platform/internal/service"

	"github.com/gogf/gf/v2/net/ghttp"
)

// GET /api/v1/developers
func ListDevelopers(r *ghttp.Request) {
	list, err := service.Developer().ListActive(r.Context())
	if err != nil {
		r.Response.WriteJson(map[string]any{
			"code":    -1,
			"message": err.Error(),
		})
		return
	}
	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": "ok",
		"data": map[string]any{
			"list": list,
		},
	})
}
