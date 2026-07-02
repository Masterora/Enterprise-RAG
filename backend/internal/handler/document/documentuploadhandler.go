// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"net/http"

	"enterprise-rag/backend/internal/logic/document"
	"enterprise-rag/backend/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func DocumentUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := document.NewDocumentUploadLogic(r.Context(), svcCtx)
		resp, err := l.DocumentUpload(r)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
