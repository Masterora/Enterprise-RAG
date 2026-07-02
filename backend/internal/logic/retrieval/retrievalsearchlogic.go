// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package retrieval

import (
	"context"
	"errors"
	"strings"

	"enterprise-rag/backend/internal/auth"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RetrievalSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRetrievalSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RetrievalSearchLogic {
	return &RetrievalSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RetrievalSearchLogic) RetrievalSearch(req *types.RetrievalSearchReq) (resp *types.RetrievalSearchResp, err error) {
	subjectID := strings.TrimSpace(req.SubjectID)
	query := strings.TrimSpace(req.Query)
	if subjectID == "" || query == "" {
		return nil, errors.New("subject_id and query are required")
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

	vectors, err := l.svcCtx.Embedder.Embed(l.ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, errors.New("query embedding is empty")
	}

	chunks, err := l.svcCtx.MilvusStore.Search(l.ctx, subjectID, vectors[0], req.TopK)
	if err != nil {
		return nil, err
	}

	list := make([]types.RetrievalChunk, 0, len(chunks))
	for _, chunk := range chunks {
		document, err := l.svcCtx.DocumentRepo.GetByID(l.ctx, chunk.DocID)
		if err != nil {
			return nil, err
		}

		list = append(list, types.RetrievalChunk{
			ID:         chunk.ID,
			DocID:      chunk.DocID,
			DocName:    document.Filename,
			SubjectID:  chunk.SubjectID,
			UserID:     chunk.UserID,
			ChunkIndex: chunk.ChunkIndex,
			Page:       chunk.Page,
			Section:    chunk.Section,
			Content:    chunk.Content,
			Score:      chunk.Score,
		})
	}

	return &types.RetrievalSearchResp{List: list}, nil
}
