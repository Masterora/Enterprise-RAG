package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/llm"
	"enterprise-rag/backend/internal/infrastructure/observability"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/attribute"
)

type indexedToolResult struct {
	call   ToolCall
	result ToolResult
	err    error
}

func (r *Runner) executeTools(ctx context.Context, cfg config.AgentConf, input Input, client llm.Client, calls []ToolCall, iteration int, callbacks Callbacks) ([]indexedToolResult, []types.AgentStep, error) {
	executeCtx, cancelExecution := context.WithCancel(ctx)
	defer cancelExecution()
	results := make([]indexedToolResult, len(calls))
	steps := make([]types.AgentStep, len(calls))
	var callbackErr error
	var callbackErrOnce sync.Once
	recordCallbackError := func(err error) {
		if err != nil {
			callbackErrOnce.Do(func() {
				callbackErr = err
				cancelExecution()
			})
		}
	}
	toolContext := ToolContext{
		UserID: input.UserID, SubjectID: input.SubjectID, Question: input.Question, TopK: input.TopK,
		WebSearch: input.WebSearch, LLMProvider: input.LLMProvider, LLMModel: input.LLMModel, LLM: client,
		ExpectedDocs: input.ExpectedDocIDs, ExpectedIDs: input.ExpectedChunkIDs,
	}
	for index, call := range calls {
		step := newStep("tool", "agent.tool."+call.Name, call.Name)
		step.ID = call.ID
		step.State = string(StateExecuting)
		step.Iteration = iteration
		steps[index] = step
		if err := emitStep(callbacks.OnStep, step); err != nil {
			return results, steps, err
		}
	}
	execute := func(index int) {
		call := calls[index]
		step := steps[index]
		startedAt := time.Now()
		tool, ok := r.registry.Get(call.Name)
		if !ok {
			results[index] = indexedToolResult{call: call, err: errors.New("agent tool is not registered")}
		} else {
			toolCtx, cancel := context.WithTimeout(executeCtx, time.Duration(cfg.ToolTimeoutSeconds)*time.Second)
			toolCtx, toolSpan := observability.StartSpan(toolCtx, "agent.tool.execute",
				attribute.String("agent.tool", call.Name),
				attribute.Int("agent.iteration", iteration),
			)
			if r.svcCtx != nil {
				toolCtx = r.svcCtx.Metrics.ModelUsageContext(toolCtx, "llm", "agent_tool_"+call.Name,
					resolvedProvider(input.LLMProvider, r.svcCtx.Config.LLM.Provider))
			}
			result, err := tool.Execute(toolCtx, toolContext, call.Arguments)
			observability.EndSpan(toolSpan, err)
			cancel()
			results[index] = indexedToolResult{call: call, result: result, err: err}
		}
		step.DurationMS = time.Since(startedAt).Milliseconds()
		if results[index].err != nil {
			step.Status = "failed"
			logx.WithContext(ctx).Errorf("agent tool failed: tool=%s call_id=%s err=%v", call.Name, call.ID, results[index].err)
		} else {
			step.Status = "completed"
			detail := results[index].result.Summary
			if detail == "" {
				detail = results[index].result.Content
			}
			step.Detail = truncateRunes(detail, cfg.MaxStepDetailRunes)
		}
		if r.svcCtx != nil {
			r.svcCtx.Metrics.ObserveTool(call.Name, outcome(results[index].err), time.Since(startedAt))
		}
		steps[index] = step
		recordCallbackError(emitStep(callbacks.OnStep, step))
	}
	if cfg.ParallelTools && len(calls) > 1 {
		var waitGroup sync.WaitGroup
		waitGroup.Add(len(calls))
		for index := range calls {
			go func() {
				defer waitGroup.Done()
				execute(index)
			}()
		}
		waitGroup.Wait()
	} else {
		for index := range calls {
			execute(index)
		}
	}
	return results, steps, callbackErr
}

func validatePlan(calls []ToolCall, specs []ToolSpec, limit, maxArgumentRunes int) []ToolCall {
	return validatePlanWithHistory(calls, specs, limit, maxArgumentRunes, nil)
}

func validatePlanWithHistory(calls []ToolCall, specs []ToolSpec, limit, maxArgumentRunes int, history map[string]struct{}) []ToolCall {
	allowed := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		allowed[normalizeToolName(spec.Name)] = struct{}{}
	}
	seen := make(map[string]struct{})
	validated := make([]ToolCall, 0, minInt(len(calls), limit))
	for _, call := range calls {
		call.Name = normalizeToolName(call.Name)
		if _, ok := allowed[call.Name]; !ok || len([]rune(string(call.Arguments))) > maxArgumentRunes {
			continue
		}
		arguments, ok := canonicalArguments(call.Arguments)
		if !ok {
			continue
		}
		call.Arguments = arguments
		key := call.Name + ":" + string(arguments)
		if _, exists := seen[key]; exists {
			continue
		}
		if _, exists := history[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if history != nil {
			history[key] = struct{}{}
		}
		validated = append(validated, call)
		if len(validated) >= limit {
			break
		}
	}
	return validated
}

func canonicalArguments(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	encoded, err := json.Marshal(value)
	return encoded, err == nil
}

func fallbackPlan(question string, specs []ToolSpec) Plan {
	name := ""
	for _, spec := range specs {
		if spec.Name == ToolKnowledgeSearch {
			name = spec.Name
			break
		}
	}
	if name == "" && len(specs) > 0 {
		name = specs[0].Name
	}
	if name == "" {
		return Plan{Goal: "直接回答"}
	}
	arguments, _ := json.Marshal(map[string]any{"query": question})
	if name == ToolDocumentNavigation {
		arguments, _ = json.Marshal(map[string]any{"topic": question})
	} else if name != ToolKnowledgeSearch && name != ToolWebSearch {
		arguments = json.RawMessage(`{}`)
	}
	return Plan{Goal: "检索并回答用户问题", Tools: []ToolCall{{ID: "fallback-1", Name: name, Arguments: arguments}}}
}
