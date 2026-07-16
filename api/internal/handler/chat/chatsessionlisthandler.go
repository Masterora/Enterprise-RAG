// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"net/http"

	"enterprise-rag/api/internal/logic/chat"
	"enterprise-rag/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ChatSessionListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := chat.NewChatSessionListLogic(r.Context(), svcCtx)
		resp, err := l.ChatSessionList()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
