package observability

import "context"

type ModelUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
}

type modelUsageObserverKey struct{}

func WithModelUsageObserver(ctx context.Context, observer func(ModelUsage)) context.Context {
	if observer == nil {
		return ctx
	}
	return context.WithValue(ctx, modelUsageObserverKey{}, observer)
}

func ReportModelUsage(ctx context.Context, usage ModelUsage) {
	observer, ok := ctx.Value(modelUsageObserverKey{}).(func(ModelUsage))
	if ok {
		observer(usage)
	}
}
