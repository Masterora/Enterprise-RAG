package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/infrastructure/observability"
	"enterprise-rag/backend/internal/service/chatflow"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"go.opentelemetry.io/otel/attribute"
)

type Runner struct {
	svcCtx   *svc.ServiceContext
	registry *Registry
}

func NewRunner(svcCtx *svc.ServiceContext) (*Runner, error) {
	registry, err := NewDefaultRegistry(svcCtx)
	if err != nil {
		return nil, err
	}
	return &Runner{svcCtx: svcCtx, registry: registry}, nil
}

func (r *Runner) Run(ctx context.Context, input Input, callbacks Callbacks, stream bool) (Output, error) {
	startedAt := time.Now()
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.SubjectID) == "" || strings.TrimSpace(input.Question) == "" {
		return Output{}, errors.New("agent requires user, knowledge base and question")
	}

	config := normalizeAgentConfig(r.svcCtx.Config.Agent)
	if !config.Enabled {
		return Output{}, errors.New("agent runtime is disabled")
	}
	if len([]rune(input.Question)) > config.MaxQuestionRunes {
		return Output{}, errors.New("agent question exceeds configured limit")
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(config.TimeoutSeconds)*time.Second)
	defer cancel()
	runCtx, runSpan := observability.StartSpan(runCtx, "agent.run",
		attribute.Bool("agent.stream", stream),
		attribute.Bool("agent.web_search", input.WebSearch),
	)
	runStatus := "error"
	iterations := 0
	machine := NewStateMachine()
	machine.SetObserver(func(from, to State) {
		r.svcCtx.Metrics.ObserveAgentTransition(string(from), string(to))
	})
	r.svcCtx.Metrics.AgentStarted()
	defer func() {
		if runStatus != "success" && machine.State() != StateFailed && machine.State() != StateCompleted {
			_ = machine.Transition(StateFailed)
		}
		r.svcCtx.Metrics.AgentFinished()
		if runCtx.Err() == context.DeadlineExceeded {
			runStatus = "timeout"
		}
		r.svcCtx.Metrics.ObserveAgentRun(runStatus, stream, time.Since(startedAt), iterations)
		observability.EndSpanWithStatus(runSpan, runStatus)
	}()

	client, err := chatflow.ResolveLLM(runCtx, r.svcCtx, &types.ChatAskReq{
		LlmProvider: input.LLMProvider,
		LlmModel:    input.LLMModel,
	})
	if err != nil {
		return Output{}, err
	}

	allowed := enabledToolSet(config.EnabledTools)
	if !input.WebSearch {
		delete(allowed, ToolWebSearch)
	}
	specs := r.registry.Specs(allowed)
	if len(specs) == 0 {
		return Output{}, errors.New("agent has no enabled tools")
	}

	steps := make([]types.AgentStep, 0, config.MaxTotalTools+config.MaxIterations*2+1)
	plans := make([]Plan, 0, config.MaxIterations)
	results := make([]indexedToolResult, 0, config.MaxTotalTools)
	seenCalls := make(map[string]struct{}, config.MaxTotalTools)

	for machine.Iteration() < config.MaxIterations && len(results) < config.MaxTotalTools {
		if err := machine.StartIteration(); err != nil {
			return Output{Steps: steps}, err
		}
		iteration := machine.Iteration()
		iterations = iteration
		planningStep := newStep("planning", "agent.plan", "")
		planningStep.State = string(machine.State())
		planningStep.Iteration = iteration
		if err := emitStep(callbacks.OnStep, planningStep); err != nil {
			return Output{Steps: steps}, err
		}
		if err := emitStatus(callbacks.OnStatus, "agent.plan.start"); err != nil {
			return Output{Steps: steps}, err
		}

		planStarted := time.Now()
		planCtx, planSpan := observability.StartSpan(runCtx, "agent.plan",
			attribute.Int("agent.iteration", iteration),
		)
		planCtx = r.svcCtx.Metrics.ModelUsageContext(planCtx, "llm", "agent_plan",
			resolvedProvider(input.LLMProvider, r.svcCtx.Config.LLM.Provider))
		planPrompt, err := BuildPlanPrompt(r.svcCtx.Config.Prompt, input.Question, specs,
			formatToolHistory(plans), formatPlanningObservations(results, config.MaxObservationRunes))
		if err != nil {
			observability.EndSpan(planSpan, err)
			_ = machine.Transition(StateFailed)
			return Output{Steps: steps}, err
		}
		modelStartedAt := time.Now()
		planRaw, planErr := chatflow.GenerateAnswer(planCtx, client, r.svcCtx.Config.Reliability, planPrompt)
		r.svcCtx.Metrics.ObserveModel("llm", "agent_plan", resolvedProvider(input.LLMProvider, r.svcCtx.Config.LLM.Provider), outcome(planErr), time.Since(modelStartedAt))
		plan, parseErr := ParsePlan(planRaw)
		spanErr := planErr
		if spanErr == nil {
			spanErr = parseErr
		}
		observability.EndSpan(planSpan, spanErr)
		if planErr != nil || parseErr != nil {
			if iteration == 1 {
				plan = fallbackPlan(input.Question, specs)
			} else {
				plan = Plan{Goal: "基于已有结果生成回答", Done: true, Reason: "后续规划失败"}
			}
			logx.WithContext(ctx).Errorf("agent planning fallback: iteration=%d generate_err=%v parse_err=%v", iteration, planErr, parseErr)
		}
		remainingTools := config.MaxTotalTools - len(results)
		plan.Tools = validatePlanWithHistory(plan.Tools, specs, minInt(config.MaxTools, remainingTools), config.MaxArgumentRunes, seenCalls)
		plans = append(plans, plan)

		planningStep.Status = "completed"
		planningStep.Detail = truncateRunes(planDetail(plan), config.MaxStepDetailRunes)
		planningStep.DurationMS = time.Since(planStarted).Milliseconds()
		steps = append(steps, planningStep)
		if err := emitStep(callbacks.OnStep, planningStep); err != nil {
			return Output{Steps: steps}, err
		}

		if plan.Done || len(plan.Tools) == 0 {
			if err := machine.Transition(StateSynthesizing); err != nil {
				return Output{Steps: steps}, err
			}
			break
		}
		if err := machine.Transition(StateExecuting); err != nil {
			return Output{Steps: steps}, err
		}
		iterationResults, toolSteps, err := r.executeTools(runCtx, config, input, client, plan.Tools, iteration, callbacks)
		steps = append(steps, toolSteps...)
		if err != nil {
			_ = machine.Transition(StateFailed)
			return Output{Steps: steps}, err
		}
		results = append(results, iterationResults...)
		if err := machine.Transition(StateObserving); err != nil {
			return Output{Steps: steps}, err
		}

		observationStep := newStep("observation", "agent.observe", "")
		observationStep.State = string(machine.State())
		observationStep.Iteration = iteration
		observationStep.Status = "completed"
		observationStep.Detail = observationDetail(iterationResults)
		steps = append(steps, observationStep)
		if err := emitStep(callbacks.OnStep, observationStep); err != nil {
			return Output{Steps: steps}, err
		}
		if !hasSuccessfulResult(iterationResults) {
			if err := machine.Transition(StateSynthesizing); err != nil {
				return Output{Steps: steps}, err
			}
			break
		}
	}
	if machine.State() == StateObserving || machine.State() == StatePlanning {
		if err := machine.Transition(StateSynthesizing); err != nil {
			return Output{Steps: steps}, err
		}
	}

	chunks, links, metrics := collectToolResults(results)
	if err := emitSources(callbacks.OnSources, chunks); err != nil {
		return Output{}, err
	}
	if err := emitWebSources(callbacks.OnWebSources, links); err != nil {
		return Output{}, err
	}

	synthesisStep := newStep("synthesis", "agent.answer", "")
	synthesisStep.State = string(machine.State())
	synthesisStep.Iteration = machine.Iteration()
	if err := emitStep(callbacks.OnStep, synthesisStep); err != nil {
		return Output{}, err
	}
	if err := emitStatus(callbacks.OnStatus, "agent.answer.start"); err != nil {
		return Output{}, err
	}
	synthesisStarted := time.Now()
	synthesisCtx, synthesisSpan := observability.StartSpan(runCtx, "agent.synthesize",
		attribute.Int("agent.iterations", iterations),
		attribute.Int("agent.source_count", len(chunks)+len(links)),
	)
	synthesisCtx = r.svcCtx.Metrics.ModelUsageContext(synthesisCtx, "llm", "agent_answer",
		resolvedProvider(input.LLMProvider, r.svcCtx.Config.LLM.Provider))
	prompt := BuildAnswerPrompt(r.svcCtx.Config.Prompt, input.Question,
		formatObservations(results, config.MaxObservationRunes), formatChunks(chunks), formatLinks(links))

	var answer string
	if stream {
		var builder strings.Builder
		modelStartedAt := time.Now()
		err = chatflow.StreamAnswer(synthesisCtx, client, r.svcCtx.Config.Reliability, prompt, func(delta string) error {
			builder.WriteString(delta)
			if callbacks.OnDelta != nil {
				return callbacks.OnDelta(delta)
			}
			return nil
		})
		r.svcCtx.Metrics.ObserveModel("llm", "agent_answer", resolvedProvider(input.LLMProvider, r.svcCtx.Config.LLM.Provider), outcome(err), time.Since(modelStartedAt))
		answer = builder.String()
	} else {
		modelStartedAt := time.Now()
		answer, err = chatflow.GenerateAnswer(synthesisCtx, client, r.svcCtx.Config.Reliability, prompt)
		r.svcCtx.Metrics.ObserveModel("llm", "agent_answer", resolvedProvider(input.LLMProvider, r.svcCtx.Config.LLM.Provider), outcome(err), time.Since(modelStartedAt))
	}
	if err != nil {
		observability.EndSpan(synthesisSpan, err)
		_ = machine.Transition(StateFailed)
		synthesisStep.Status = "failed"
		synthesisStep.State = string(machine.State())
		synthesisStep.Detail = chatflow.GenerationFailureKind(err)
		synthesisStep.DurationMS = time.Since(synthesisStarted).Milliseconds()
		steps = append(steps, synthesisStep)
		_ = emitStep(callbacks.OnStep, synthesisStep)
		return Output{Steps: steps}, err
	}

	answer = chatflow.NormalizeAnswerText(answer)
	generatedFallback := answer == ""
	if generatedFallback {
		answer = "无法确定"
	}
	answer, chunks, links = chatflow.RemapReferencedSources(answer, chunks, links)
	synthesisSpan.SetAttributes(
		attribute.Int("agent.citation_count", len(chunks)+len(links)),
		attribute.Int("agent.answer_runes", len([]rune(answer))),
	)
	observability.EndSpan(synthesisSpan, nil)
	if generatedFallback && stream && callbacks.OnDelta != nil {
		if err := callbacks.OnDelta(answer); err != nil {
			return Output{}, err
		}
	}
	metrics = completeMetrics(metrics, plans, input, answer, len(chunks)+len(links), startedAt, r.svcCtx.Config.Evaluation)
	if err := emitMetrics(callbacks.OnMetrics, metrics); err != nil {
		return Output{}, err
	}

	synthesisStep.Status = "completed"
	synthesisStep.DurationMS = time.Since(synthesisStarted).Milliseconds()
	if err := machine.Transition(StateCompleted); err != nil {
		return Output{Steps: steps}, err
	}
	synthesisStep.State = string(machine.State())
	steps = append(steps, synthesisStep)
	if err := emitStep(callbacks.OnStep, synthesisStep); err != nil {
		return Output{}, err
	}
	runStatus = "success"
	return Output{Answer: answer, Chunks: chunks, ExternalLinks: links, Metrics: metrics, Steps: steps}, nil
}

func normalizeAgentConfig(value config.AgentConf) config.AgentConf {
	if value.MaxIterations < 1 {
		value.MaxIterations = 3
	}
	if value.MaxIterations > 6 {
		value.MaxIterations = 6
	}
	if value.MaxTools < 1 {
		value.MaxTools = 3
	}
	if value.MaxTools > 8 {
		value.MaxTools = 8
	}
	if value.MaxTotalTools < 1 {
		value.MaxTotalTools = value.MaxTools * 2
	} else if value.MaxTotalTools < value.MaxTools {
		value.MaxTotalTools = value.MaxTools
	}
	if value.MaxTotalTools > 16 {
		value.MaxTotalTools = 16
	}
	if value.TimeoutSeconds < 1 {
		value.TimeoutSeconds = 75
	}
	if value.ToolTimeoutSeconds < 1 {
		value.ToolTimeoutSeconds = 25
	}
	if value.MaxObservationRunes < 1000 {
		value.MaxObservationRunes = 12000
	}
	if value.MaxQuestionRunes < 1 {
		value.MaxQuestionRunes = 4000
	}
	if value.MaxArgumentRunes < 1 {
		value.MaxArgumentRunes = 1000
	}
	if value.MaxStepDetailRunes < 1 {
		value.MaxStepDetailRunes = 600
	}
	return value
}

func formatToolHistory(plans []Plan) string {
	if len(plans) == 0 {
		return ""
	}
	lines := make([]string, 0)
	for _, plan := range plans {
		for _, call := range plan.Tools {
			lines = append(lines, call.Name+":"+string(call.Arguments))
		}
	}
	return strings.Join(lines, "\n")
}

func planDetail(plan Plan) string {
	detail := strings.TrimSpace(plan.Goal)
	if reason := strings.TrimSpace(plan.Reason); reason != "" {
		if detail != "" {
			detail += "："
		}
		detail += reason
	}
	return detail
}

func observationDetail(results []indexedToolResult) string {
	succeeded := 0
	failed := 0
	for _, result := range results {
		if result.err == nil {
			succeeded++
		} else {
			failed++
		}
	}
	return fmt.Sprintf("工具执行完成：成功 %d，失败 %d", succeeded, failed)
}

func hasSuccessfulResult(results []indexedToolResult) bool {
	for _, result := range results {
		if result.err == nil {
			return true
		}
	}
	return false
}

func outcome(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "error"
}

func resolvedProvider(requested, fallback string) string {
	if requested = strings.TrimSpace(requested); requested != "" {
		return requested
	}
	return fallback
}

func enabledToolSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		names = []string{ToolKnowledgeSearch, ToolKnowledgeOverview, ToolDocumentNavigation, ToolWebSearch}
	}
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		if normalized := normalizeToolName(name); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func emitStatus(callback func(string) error, value string) error {
	if callback == nil {
		return nil
	}
	return callback(value)
}

func emitStep(callback func(types.AgentStep) error, value types.AgentStep) error {
	if callback == nil {
		return nil
	}
	return callback(value)
}

func emitSources(callback func([]types.RetrievalChunk) error, value []types.RetrievalChunk) error {
	if callback == nil || len(value) == 0 {
		return nil
	}
	return callback(value)
}

func emitWebSources(callback func([]types.ExternalLink) error, value []types.ExternalLink) error {
	if callback == nil || len(value) == 0 {
		return nil
	}
	return callback(value)
}

func emitMetrics(callback func(types.RetrievalMetrics) error, value types.RetrievalMetrics) error {
	if callback == nil {
		return nil
	}
	return callback(value)
}
