package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"enterprise-rag/backend/internal/config"
	"enterprise-rag/backend/internal/svc"
	"enterprise-rag/backend/internal/types"
)

type testLLM struct {
	responses []string
	index     int
}

func (c *testLLM) Generate(context.Context, string, bool) (string, error) {
	if c.index >= len(c.responses) {
		return "", errors.New("no test response")
	}
	response := c.responses[c.index]
	c.index++
	return response, nil
}

func (c *testLLM) GenerateStream(_ context.Context, _ string, _ bool, onDelta func(string) error) error {
	response, err := c.Generate(context.Background(), "", false)
	if err != nil {
		return err
	}
	return onDelta(response)
}

func (c *testLLM) SearchWeb(context.Context, string) ([]types.ExternalLink, error) {
	return nil, nil
}

type testTool struct {
	name      string
	delay     time.Duration
	active    *atomic.Int32
	maxActive *atomic.Int32
}

func (t testTool) Spec() ToolSpec {
	return ToolSpec{Name: t.name, Description: "test tool", Parameters: objectSchema(nil)}
}

func (t testTool) Execute(ctx context.Context, _ ToolContext, _ json.RawMessage) (ToolResult, error) {
	if t.active != nil {
		current := t.active.Add(1)
		defer t.active.Add(-1)
		for {
			maximum := t.maxActive.Load()
			if current <= maximum || t.maxActive.CompareAndSwap(maximum, current) {
				break
			}
		}
	}
	if t.delay > 0 {
		select {
		case <-time.After(t.delay):
		case <-ctx.Done():
			return ToolResult{}, ctx.Err()
		}
	}
	return ToolResult{Content: "ok"}, nil
}

func TestParsePlanAcceptsJSONFence(t *testing.T) {
	plan, err := ParsePlan("```json\n{\"goal\":\"查找资料\",\"tools\":[{\"name\":\"knowledge_search\",\"arguments\":{\"query\":\"测试\"}}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Goal != "查找资料" || len(plan.Tools) != 1 || plan.Tools[0].ID != "call-1" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestRegistryRejectsDuplicateTool(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(testTool{name: "sample"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(testTool{name: "SAMPLE"}); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestValidatePlanFiltersUnknownDuplicateAndLimit(t *testing.T) {
	specs := []ToolSpec{{Name: "a"}, {Name: "b"}}
	calls := []ToolCall{
		{ID: "1", Name: "a", Arguments: json.RawMessage(`{"x":1}`)},
		{ID: "2", Name: "a", Arguments: json.RawMessage(`{"x":1}`)},
		{ID: "3", Name: "unknown", Arguments: json.RawMessage(`{}`)},
		{ID: "4", Name: "b", Arguments: json.RawMessage(`{}`)},
	}
	validated := validatePlan(calls, specs, 2, 100)
	if len(validated) != 2 || validated[0].ID != "1" || validated[1].ID != "4" {
		t.Fatalf("unexpected validated plan: %#v", validated)
	}
}

func TestValidatePlanCanonicalizesArgumentsBeforeDeduplication(t *testing.T) {
	calls := []ToolCall{
		{ID: "1", Name: "sample", Arguments: json.RawMessage(`{"query":"test","top_k":5}`)},
		{ID: "2", Name: "sample", Arguments: json.RawMessage(`{"top_k":5,"query":"test"}`)},
		{ID: "3", Name: "sample", Arguments: json.RawMessage(`{} {}`)},
	}
	validated := validatePlan(calls, []ToolSpec{{Name: "sample"}}, 3, 100)
	if len(validated) != 1 || validated[0].ID != "1" {
		t.Fatalf("unexpected validated plan: %#v", validated)
	}
}

func TestDecodeArgumentsRejectsMultipleValues(t *testing.T) {
	var target struct{}
	if err := decodeArguments(json.RawMessage(`{} {}`), &target); err == nil {
		t.Fatal("expected multiple JSON values to be rejected")
	}
}

func TestRunnerAnswersDirectlyAndRecordsSteps(t *testing.T) {
	client := &testLLM{responses: []string{
		`{"goal":"回应用户","tools":[]}`,
		"你好，我可以帮助你查询当前知识库。",
	}}
	serviceContext := &svc.ServiceContext{
		Config: config.Config{
			Agent: config.AgentConf{
				Enabled: true, MaxTools: 3, TimeoutSeconds: 5, ToolTimeoutSeconds: 2,
				MaxQuestionRunes: 1000, MaxArgumentRunes: 1000, MaxObservationRunes: 2000,
			},
			LLM: config.ProviderConf{Provider: "openrouter", Model: "test"},
		},
		LLM: client,
	}
	runner, err := NewRunner(serviceContext)
	if err != nil {
		t.Fatal(err)
	}
	output, err := runner.Run(context.Background(), Input{
		UserID: "user", SubjectID: "subject", Question: "你好", LLMProvider: "openrouter", LLMModel: "test",
	}, Callbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.Answer, "帮助") || len(output.Steps) != 2 {
		t.Fatalf("unexpected output: %#v", output)
	}
	for _, step := range output.Steps {
		if step.Status != "completed" {
			t.Fatalf("step not completed: %#v", step)
		}
	}
}

func TestExecuteToolsRunsIndependentCallsInParallel(t *testing.T) {
	var active atomic.Int32
	var maxActive atomic.Int32
	registry := NewRegistry()
	if err := registry.Register(testTool{name: "parallel", delay: 30 * time.Millisecond, active: &active, maxActive: &maxActive}); err != nil {
		t.Fatal(err)
	}
	runner := &Runner{registry: registry}
	calls := []ToolCall{
		{ID: "1", Name: "parallel", Arguments: json.RawMessage(`{"query":"a"}`)},
		{ID: "2", Name: "parallel", Arguments: json.RawMessage(`{"query":"b"}`)},
	}
	results, steps, err := runner.executeTools(context.Background(), config.AgentConf{
		ParallelTools: true, ToolTimeoutSeconds: 1,
	}, Input{}, nil, calls, 1, Callbacks{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || len(steps) != 2 || maxActive.Load() < 2 {
		t.Fatalf("tools did not run concurrently: results=%d steps=%d max=%d", len(results), len(steps), maxActive.Load())
	}
}

func TestExecuteToolsStopsWhenStepCallbackFails(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(testTool{name: "sample"}); err != nil {
		t.Fatal(err)
	}
	runner := &Runner{registry: registry}
	expected := errors.New("client disconnected")
	_, _, err := runner.executeTools(context.Background(), config.AgentConf{ToolTimeoutSeconds: 1}, Input{}, nil,
		[]ToolCall{{ID: "1", Name: "sample", Arguments: json.RawMessage(`{}`)}},
		1, Callbacks{OnStep: func(types.AgentStep) error { return expected }})
	if !errors.Is(err, expected) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestStateMachineRejectsInvalidTransition(t *testing.T) {
	machine := NewStateMachine()
	if err := machine.Transition(StateExecuting); err == nil {
		t.Fatal("expected invalid transition error")
	}
	if err := machine.StartIteration(); err != nil {
		t.Fatal(err)
	}
	if machine.State() != StatePlanning || machine.Iteration() != 1 {
		t.Fatalf("unexpected machine state: state=%s iteration=%d", machine.State(), machine.Iteration())
	}
}

func TestRunnerReplansAfterObservation(t *testing.T) {
	client := &testLLM{responses: []string{
		`{"goal":"先检索","done":false,"tools":[{"id":"call-1","name":"sample","arguments":{"query":"a"}}]}`,
		`{"goal":"已有足够资料","done":true,"tools":[]}`,
		"根据检索结果回答。",
	}}
	registry := NewRegistry()
	if err := registry.Register(testTool{name: "sample"}); err != nil {
		t.Fatal(err)
	}
	serviceContext := &svc.ServiceContext{
		Config: config.Config{
			Agent: config.AgentConf{
				Enabled: true, MaxIterations: 3, MaxTools: 2, MaxTotalTools: 4,
				TimeoutSeconds: 5, ToolTimeoutSeconds: 2, MaxQuestionRunes: 1000,
				MaxArgumentRunes: 1000, MaxObservationRunes: 2000, EnabledTools: []string{"sample"},
			},
			LLM: config.ProviderConf{Provider: "openrouter", Model: "test"},
		},
		LLM: client,
	}
	runner := &Runner{svcCtx: serviceContext, registry: registry}
	output, err := runner.Run(context.Background(), Input{
		UserID: "user", SubjectID: "subject", Question: "测试", LLMProvider: "openrouter", LLMModel: "test",
	}, Callbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if output.Answer == "" || client.index != 3 {
		t.Fatalf("unexpected output: %#v llm_calls=%d", output, client.index)
	}
	if len(output.Steps) != 5 {
		t.Fatalf("steps=%d, want planning/tool/observation/planning/synthesis", len(output.Steps))
	}
	if output.Steps[2].State != string(StateObserving) || output.Steps[3].Iteration != 2 {
		t.Fatalf("unexpected state steps: %#v", output.Steps)
	}
}

func TestRunnerStopsAtConfiguredIterationLimit(t *testing.T) {
	client := &testLLM{responses: []string{
		`{"goal":"第一轮","done":false,"tools":[{"id":"call-1","name":"sample","arguments":{"query":"a"}}]}`,
		`{"goal":"第二轮","done":false,"tools":[{"id":"call-2","name":"sample","arguments":{"query":"b"}}]}`,
		"已根据两轮结果生成回答。",
	}}
	registry := NewRegistry()
	if err := registry.Register(testTool{name: "sample"}); err != nil {
		t.Fatal(err)
	}
	serviceContext := &svc.ServiceContext{
		Config: config.Config{
			Agent: config.AgentConf{
				Enabled: true, MaxIterations: 2, MaxTools: 1, MaxTotalTools: 4,
				TimeoutSeconds: 5, ToolTimeoutSeconds: 2, MaxQuestionRunes: 1000,
				MaxArgumentRunes: 1000, MaxObservationRunes: 2000, EnabledTools: []string{"sample"},
			},
			LLM: config.ProviderConf{Provider: "openrouter", Model: "test"},
		},
		LLM: client,
	}
	output, err := (&Runner{svcCtx: serviceContext, registry: registry}).Run(context.Background(), Input{
		UserID: "user", SubjectID: "subject", Question: "测试", LLMProvider: "openrouter", LLMModel: "test",
	}, Callbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if client.index != 3 || len(output.Steps) != 7 || output.Steps[len(output.Steps)-1].State != string(StateCompleted) {
		t.Fatalf("iteration limit was not enforced: calls=%d steps=%#v", client.index, output.Steps)
	}
}

func TestValidatePlanFiltersCallsSeenInPreviousIterations(t *testing.T) {
	history := map[string]struct{}{
		`sample:{"query":"a"}`: {},
	}
	calls := []ToolCall{
		{ID: "1", Name: "sample", Arguments: json.RawMessage(`{"query":"a"}`)},
		{ID: "2", Name: "sample", Arguments: json.RawMessage(`{"query":"b"}`)},
	}
	validated := validatePlanWithHistory(calls, []ToolSpec{{Name: "sample"}}, 2, 100, history)
	if len(validated) != 1 || validated[0].ID != "2" {
		t.Fatalf("unexpected validated plan: %#v", validated)
	}
}

func BenchmarkParsePlan(b *testing.B) {
	raw := `{"goal":"检索并回答","tools":[{"id":"call-1","name":"knowledge_search","arguments":{"query":"企业知识检索"}},{"id":"call-2","name":"web_search","arguments":{"query":"公开资料"}}]}`
	b.ReportAllocs()
	for range b.N {
		if _, err := ParsePlan(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidatePlan(b *testing.B) {
	specs := []ToolSpec{{Name: ToolKnowledgeSearch}, {Name: ToolKnowledgeOverview}, {Name: ToolWebSearch}}
	calls := []ToolCall{
		{ID: "1", Name: ToolKnowledgeSearch, Arguments: json.RawMessage(`{"query":"企业知识检索"}`)},
		{ID: "2", Name: ToolKnowledgeOverview, Arguments: json.RawMessage(`{}`)},
		{ID: "3", Name: ToolWebSearch, Arguments: json.RawMessage(`{"query":"公开资料"}`)},
	}
	b.ReportAllocs()
	for range b.N {
		if len(validatePlan(calls, specs, 3, 1000)) != 3 {
			b.Fatal("unexpected validated plan")
		}
	}
}
