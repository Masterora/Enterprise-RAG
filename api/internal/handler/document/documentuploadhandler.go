// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"errors"
	"net/http"

	"enterprise-rag/api/internal/logic/document"
	"enterprise-rag/api/internal/service/documentupload"
	"enterprise-rag/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func DocumentUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, documentupload.MaxRequestSize)
		l := document.NewDocumentUploadLogic(r.Context(), svcCtx)
		resp, err := l.DocumentUpload(r)
		if err != nil {
			if errors.Is(err, documentupload.ErrFileTooLarge) {
				httpx.WriteJsonCtx(r.Context(), w, http.StatusRequestEntityTooLarge, map[string]string{
					"message": err.Error(),
				})
				return
			}
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
