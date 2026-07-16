// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package subject

import (
	"net/http"

	"enterprise-rag/api/internal/logic/subject"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func SubjectCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SubjectCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := subject.NewSubjectCreateLogic(r.Context(), svcCtx)
		resp, err := l.SubjectCreate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
