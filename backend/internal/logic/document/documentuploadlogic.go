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
	documentpresenter "enterprise-rag/backend/internal/presenter/document"
	"enterprise-rag/backend/internal/service/documentupload"
	"enterprise-rag/backend/internal/service/taskqueue"
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
	input, err := documentupload.ParseRequest(r)
	if err != nil {
		return nil, err
	}
	defer input.File.Close()

	subjectID := strings.TrimSpace(input.SubjectID)
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

	docID := uuid.NewString()
	filename := filepath.Base(input.Filename)
	if filename == "." || filename == string(filepath.Separator) {
		return nil, errors.New("filename is invalid")
	}
	duplicate, err := l.svcCtx.DocumentRepo.ExistsActiveFilename(l.ctx, user.ID, subjectID, filename)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, errors.New("document filename already exists")
	}

	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext == "" {
		ext = "unknown"
	}
	processingMode := task.NormalizeProcessingMode(input.ProcessingMode)

	objectName := fmt.Sprintf("documents/%s/%s", docID, filename)
	_, err = l.svcCtx.MinIO.PutObject(
		l.ctx,
		l.svcCtx.Config.MinIO.Bucket,
		objectName,
		input.File,
		input.FileSize,
		minio.PutObjectOptions{ContentType: input.ContentType},
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
		FileSize:  input.FileSize,
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
	indexTask.Metadata, err = json.Marshal(task.ParseTaskMetadata{ProcessingMode: processingMode})
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.DocumentRepo.CreateWithIndexTask(l.ctx, documentModel, indexTask); err != nil {
		return nil, err
	}

	if err := taskqueue.Publish(l.svcCtx.Nats, model.TaskTypeParse, indexTask.ID, docID, processingMode); err != nil {
		_ = l.svcCtx.IndexTaskRepo.UpdateStatus(l.ctx, indexTask.ID, model.TaskStatusFailed, err.Error())
		_ = l.svcCtx.DocumentRepo.UpdateStatus(l.ctx, docID, model.DocumentStatusFailed, err.Error())
		return nil, err
	}

	return &types.DocumentUploadResp{
		Document: documentpresenter.ToInfo(*documentModel),
	}, nil
}
