package admin

import (
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// storeNote appends a note to ref_type when present, so notes survive in the
// transactions table without a schema change.
func storeNote(refType, note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return refType
	}
	return refType + ": " + note
}

// gfast-compatible response helpers. All responses use HTTP 200; errors are
// indicated by the JSON code field (-1 on failure).

func ok(r *ghttp.Request, data any) {
	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": "ok",
		"data":    data,
	})
}

func okMsg(r *ghttp.Request, msg string) {
	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": msg,
	})
}

func fail(r *ghttp.Request, msg string) {
	r.Response.WriteJson(map[string]any{
		"code":    -1,
		"message": msg,
	})
}

func page(r *ghttp.Request, list any, total int, pageNum, pageSize int) {
	r.Response.WriteJson(map[string]any{
		"code":    0,
		"message": "ok",
		"data": map[string]any{
			"list":     list,
			"total":    total,
			"pageNum":  pageNum,
			"pageSize": pageSize,
		},
	})
}
