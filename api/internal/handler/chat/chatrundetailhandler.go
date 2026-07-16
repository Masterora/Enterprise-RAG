// Code scaffolded by goctl. Safe to edit.
package chat

import (
	"net/http"

	chatlogic "enterprise-rag/api/internal/logic/chat"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ChatRunDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatRunReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := chatlogic.NewChatRunDetailLogic(r.Context(), svcCtx).ChatRunDetail(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
