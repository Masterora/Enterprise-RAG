// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package settings

import (
	"errors"
	"net/http"

	settingslogic "enterprise-rag/api/internal/logic/settings"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ModelServiceSettingsUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ModelServiceSettingsUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		logic := settingslogic.NewModelServiceSettingsLogic(r.Context(), svcCtx)
		resp, err := logic.Update(&req)
		if errors.Is(err, settingslogic.ErrInvalidAPIKey) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"API Key 校验失败，请确认 Key 有效且账户额度充足"}`))
			return
		}
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}
