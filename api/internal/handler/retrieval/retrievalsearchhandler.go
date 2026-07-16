// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package retrieval

import (
	"net/http"

	"enterprise-rag/api/internal/logic/retrieval"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RetrievalSearchHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RetrievalSearchReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := retrieval.NewRetrievalSearchLogic(r.Context(), svcCtx)
		resp, err := l.RetrievalSearch(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
