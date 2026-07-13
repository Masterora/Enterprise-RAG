package observability

import (
	"context"
	"testing"
)

func TestModelUsageObserver(t *testing.T) {
	want := ModelUsage{InputTokens: 10, OutputTokens: 2, TotalTokens: 12, CostUSD: 0.01}
	var got ModelUsage
	ctx := WithModelUsageObserver(context.Background(), func(usage ModelUsage) {
		got = usage
	})
	ReportModelUsage(ctx, want)
	if got != want {
		t.Fatalf("usage = %+v, want %+v", got, want)
	}
}
