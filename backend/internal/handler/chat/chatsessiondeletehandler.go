// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"net/http"

	"enterprise-rag/backend/internal/logic/chat"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ChatSessionDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatSessionDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := chat.NewChatSessionDeleteLogic(r.Context(), svcCtx)
		resp, err := l.ChatSessionDelete(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
