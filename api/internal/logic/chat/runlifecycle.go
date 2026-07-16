package chat

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"enterprise-rag/api/internal/auth"
	agentinfra "enterprise-rag/api/internal/infrastructure/agent"
	"enterprise-rag/api/internal/model"
	"enterprise-rag/api/internal/svc"
	"enterprise-rag/api/internal/types"

	"github.com/google/uuid"
)

func beginRun(ctx context.Context, svcCtx *svc.ServiceContext, user auth.UserSession, req *types.ChatAskReq, runID string, create bool) (context.Context, func(), error) {
	if runID == "" {
		runID = uuid.NewString()
	}
	if create {
		requestBody, err := json.Marshal(req)
		if err != nil {
			return nil, nil, err
		}
		now := time.Now()
		if err := svcCtx.RunRepo.Create(ctx, &model.ChatRun{
			ID: runID, TenantID: user.TenantID, UserID: user.ID, SessionID: req.SessionID,
			MessageID: req.MessageID, SubjectID: req.SubjectID, Status: model.RunStatusCreated,
			Request: requestBody, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return nil, nil, err
		}
	}
	if err := svcCtx.RunRepo.MarkRunning(ctx, runID, user.TenantID, user.ID); err != nil {
		return nil, nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	unregister := svcCtx.RunController.Register(runID, cancel)
	stopWatcher := watchPersistentCancellation(runCtx, svcCtx, user, runID, cancel)
	cleanup := func() {
		stopWatcher()
		unregister()
		cancel()
	}
	_ = appendRunEvent(ctx, svcCtx, user.TenantID, runID, "run.running", map[string]any{"run_id": runID})
	return runCtx, cleanup, nil
}

func watchPersistentCancellation(ctx context.Context, svcCtx *svc.ServiceContext, user auth.UserSession, runID string, cancel context.CancelFunc) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				pollCtx, pollCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
				run, err := svcCtx.RunRepo.GetForUser(pollCtx, runID, user.TenantID, user.ID)
				pollCancel()
				if err == nil && run.CancelRequested {
					cancel()
					return
				}
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

func completeRun(ctx context.Context, svcCtx *svc.ServiceContext, user auth.UserSession, runID string, output agentinfra.Result) error {
	result, err := json.Marshal(output)
	if err != nil {
		return err
	}
	terminalCtx, cancel := terminalContext(ctx)
	defer cancel()
	if err := svcCtx.RunRepo.Complete(terminalCtx, runID, user.TenantID, user.ID, result); err != nil {
		return err
	}
	_ = appendRunEvent(terminalCtx, svcCtx, user.TenantID, runID, "run.completed", map[string]any{"run_id": runID})
	_ = svcCtx.Agent.Cleanup(terminalCtx, user.TenantID, runID)
	return nil
}

func failRun(ctx context.Context, svcCtx *svc.ServiceContext, user auth.UserSession, runID string, runErr error) {
	status := model.RunStatusFailed
	if errors.Is(runErr, context.Canceled) {
		status = model.RunStatusCancelled
	}
	terminalCtx, cancel := terminalContext(ctx)
	defer cancel()
	_ = svcCtx.RunRepo.Fail(terminalCtx, runID, user.TenantID, user.ID, status, runErr.Error())
	_ = appendRunEvent(terminalCtx, svcCtx, user.TenantID, runID, "run."+status, map[string]any{"message": runErr.Error()})
}

func appendRunEvent(ctx context.Context, svcCtx *svc.ServiceContext, tenantID, runID, eventType string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = svcCtx.RunRepo.AppendEvent(ctx, &model.ChatRunEvent{
		RunID: runID, TenantID: tenantID, Type: eventType, Payload: body, CreatedAt: time.Now(),
	})
	return err
}

func terminalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func effectiveRunError(runCtx context.Context, err error) error {
	if runErr := runCtx.Err(); runErr != nil {
		return runErr
	}
	return err
}
