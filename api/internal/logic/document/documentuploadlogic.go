// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package document

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"enterprise-rag/api/internal/auth"
	"enterprise-rag/api/internal/model"
	documentpresenter "enterprise-rag/api/internal/presenter/document"
	"enterprise-rag/api/internal/service/documentupload"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/task"
	"enterprise-rag/api/internal/types"

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
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	subjectID := strings.TrimSpace(input.SubjectID)
	if subjectID == "" {
		return nil, errors.New("subject_id is required")
	}

	user, err := auth.CurrentUser(l.ctx)
	if err != nil {
		return nil, err
	}

	subject, err := l.svcCtx.SubjectRepo.GetAccessibleByID(l.ctx, subjectID, user.ID, user.TenantID)
	if err != nil {
		return nil, err
	}
	if !canWriteSubject(subject, user.ID) {
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
	hasher := sha256.New()
	if _, err := io.Copy(hasher, input.File); err != nil {
		return nil, err
	}
	if _, err := input.File.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	contentHash := fmt.Sprintf("%x", hasher.Sum(nil))

	objectName := fmt.Sprintf("tenants/%s/documents/%s/%s", user.TenantID, docID, filename)
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
		ID:                   docID,
		TenantID:             user.TenantID,
		SubjectID:            subjectID,
		UserID:               user.ID,
		Filename:             filename,
		FileType:             ext,
		FileSize:             input.FileSize,
		FileURL:              fileURL,
		Status:               model.DocumentStatusUploaded,
		DocumentVersion:      1,
		ContentHash:          contentHash,
		EmbeddingProvider:    l.svcCtx.Config.Embedding.Provider,
		EmbeddingModel:       l.svcCtx.Config.Embedding.Model,
		EmbeddingDimension:   l.svcCtx.Config.Embedding.Dimension,
		ChunkStrategyVersion: l.svcCtx.Config.Chunking.Version,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	indexTask := &model.IndexTask{
		ID:                   uuid.NewString(),
		TenantID:             user.TenantID,
		DocID:                docID,
		SubjectID:            subjectID,
		UserID:               user.ID,
		TaskType:             model.TaskTypeParse,
		Status:               model.TaskStatusPending,
		DocumentVersion:      documentModel.DocumentVersion,
		ContentHash:          documentModel.ContentHash,
		EmbeddingProvider:    documentModel.EmbeddingProvider,
		EmbeddingModel:       documentModel.EmbeddingModel,
		EmbeddingDimension:   documentModel.EmbeddingDimension,
		ChunkStrategyVersion: documentModel.ChunkStrategyVersion,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	indexTask.Metadata, err = json.Marshal(task.ParseTaskMetadata{ProcessingMode: processingMode})
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.DocumentRepo.CreateWithIndexTask(l.ctx, documentModel, indexTask); err != nil {
		return nil, err
	}

	return &types.DocumentUploadResp{
		Document: documentpresenter.ToInfo(*documentModel),
	}, nil
}

func canWriteSubject(subject *model.Subject, userID string) bool {
	return subject != nil && strings.TrimSpace(subject.OwnerID) == strings.TrimSpace(userID)
}
