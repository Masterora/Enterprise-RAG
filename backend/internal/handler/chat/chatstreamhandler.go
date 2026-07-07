// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package chat

import (
	"encoding/json"
	"net/http"

	chatlogic "enterprise-rag/backend/internal/logic/chat"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ChatStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatAskReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			httpx.ErrorCtx(r.Context(), w, http.ErrNotSupported)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		writeEvent := func(event string, payload any) error {
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			if _, err := w.Write([]byte("event: " + event + "\n")); err != nil {
				return err
			}
			if _, err := w.Write([]byte("data: " + string(body) + "\n\n")); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}

		l := chatlogic.NewChatAskLogic(r.Context(), svcCtx)
		err := l.ChatStream(&req, chatlogic.StreamCallbacks{
			OnStatus: func(message string) error {
				return writeEvent("status", map[string]string{"message": message})
			},
			OnSources: func(chunks []types.RetrievalChunk) error {
				return writeEvent("sources", map[string][]types.RetrievalChunk{"chunks": chunks})
			},
			OnDelta: func(content string) error {
				return writeEvent("delta", map[string]string{"content": content})
			},
			OnDone: func() error {
				return writeEvent("done", map[string]bool{"ok": true})
			},
		})
		if err != nil {
			_ = writeEvent("error", map[string]string{"message": err.Error()})
		}
	}
}
