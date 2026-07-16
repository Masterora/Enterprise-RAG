package document

import (
	"net/http"

	"enterprise-rag/api/internal/logic/document"
	"enterprise-rag/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func DocumentClearFailedHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := document.NewDocumentClearFailedLogic(r.Context(), svcCtx)
		resp, err := l.DocumentClearFailed()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
