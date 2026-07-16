package settings

import (
	"testing"

	"enterprise-rag/api/internal/service/modelsettings"
)

func TestConfiguredSecret(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "whitespace", value: "   ", want: false},
		{name: "placeholder", value: "replace-with-openrouter-api-key", want: false},
		{name: "configured", value: "sk-or-v1-example", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := modelsettings.IsConfiguredSecret(test.value); got != test.want {
				t.Fatalf("IsConfiguredSecret(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
