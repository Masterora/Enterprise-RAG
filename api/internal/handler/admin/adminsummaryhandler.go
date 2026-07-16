// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"net/http"

	"enterprise-rag/api/internal/logic/admin"
	"enterprise-rag/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := admin.NewAdminSummaryLogic(r.Context(), svcCtx)
		resp, err := l.AdminSummary()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
