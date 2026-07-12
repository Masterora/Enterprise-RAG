// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"net/http"

	"enterprise-rag/backend/internal/logic/admin"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminLogListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminLogListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := admin.NewAdminLogListLogic(r.Context(), svcCtx)
		resp, err := l.AdminLogList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
