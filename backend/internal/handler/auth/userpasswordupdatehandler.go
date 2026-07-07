package auth

import (
	"net/http"

	"enterprise-rag/backend/internal/logic/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UserPasswordUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UserPasswordUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := auth.NewUserPasswordUpdateLogic(r.Context(), svcCtx)
		resp, err := l.UserPasswordUpdate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
