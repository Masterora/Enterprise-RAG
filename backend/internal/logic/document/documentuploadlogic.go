// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/model"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/task"
	"enterprise-rag/backend/internal/types"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/zeromicro/go-zero/core/logx"
)

type DocumentUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDocumentUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DocumentUploadLogic {
	return &DocumentUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DocumentUploadLogic) DocumentUpload(r *http.Request) (resp *types.DocumentUploadResp, err error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}

	subjectID := strings.TrimSpace(r.FormValue("subject_id"))
	if subjectID == "" {
		return nil, errors.New("subject_id is required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	exists, err := l.svcCtx.SubjectRepo.ExistsAccessible(l.ctx, subjectID, user.ID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("knowledge base not found")
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	docID := uuid.NewString()
	filename := filepath.Base(header.Filename)
	if filename == "." || filename == string(filepath.Separator) {
		return nil, errors.New("filename is invalid")
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext == "" {
		ext = "unknown"
	}

	objectName := fmt.Sprintf("documents/%s/%s", docID, filename)
	_, err = l.svcCtx.MinIO.PutObject(
		l.ctx,
		l.svcCtx.Config.MinIO.Bucket,
		objectName,
		file,
		header.Size,
		minio.PutObjectOptions{ContentType: header.Header.Get("Content-Type")},
	)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	fileURL := fmt.Sprintf("minio://%s/%s", l.svcCtx.Config.MinIO.Bucket, objectName)
	documentModel := &model.Document{
		ID:        docID,
		SubjectID: subjectID,
		UserID:    user.ID,
		Filename:  filename,
		FileType:  ext,
		FileSize:  header.Size,
		FileURL:   fileURL,
		Status:    model.DocumentStatusUploaded,
		CreatedAt: now,
		UpdatedAt: now,
	}
	indexTask := &model.IndexTask{
		ID:        uuid.NewString(),
		DocID:     docID,
		SubjectID: subjectID,
		UserID:    user.ID,
		TaskType:  model.TaskTypeParse,
		Status:    model.TaskStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := l.svcCtx.DocumentRepo.CreateWithIndexTask(l.ctx, documentModel, indexTask); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(task.Message{DocID: docID})
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.Nats.Publish(model.TaskTypeParse, payload); err != nil {
		return nil, err
	}

	return &types.DocumentUploadResp{
		Document: toDocumentInfo(*documentModel),
	}, nil
}
