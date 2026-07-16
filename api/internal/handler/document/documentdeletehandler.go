// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"net/http"

	"enterprise-rag/api/internal/logic/document"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func DocumentDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DocumentDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := document.NewDocumentDeleteLogic(r.Context(), svcCtx)
		resp, err := l.DocumentDelete(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
