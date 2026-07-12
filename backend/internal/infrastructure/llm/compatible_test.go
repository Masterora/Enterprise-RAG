package llm

import (
	"strings"
	"testing"
)

func TestEnableWebSearch(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		enabled     bool
		wantErr     bool
		wantTool    string
		wantToolLen int
	}{
		{
			name:        "adds openrouter tool",
			baseURL:     "https://openrouter.ai/api/v1",
			enabled:     true,
			wantTool:    "openrouter:web_search",
			wantToolLen: 1,
		},
		{
			name:    "rejects unsupported provider",
			baseURL: "https://example.com/v1",
			enabled: true,
			wantErr: true,
		},
		{
			name:        "skips when disabled",
			baseURL:     "https://openrouter.ai/api/v1",
			enabled:     false,
			wantToolLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCompatibleClient("key", "model", tt.baseURL)
			payload := map[string]any{}

			err := client.enableWebSearch(payload, tt.enabled)
			if (err != nil) != tt.wantErr {
				t.Fatalf("enableWebSearch() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			tools, _ := payload["tools"].([]map[string]string)
			if len(tools) != tt.wantToolLen {
				t.Fatalf("tool count = %d, want %d", len(tools), tt.wantToolLen)
			}
			if tt.wantToolLen > 0 && tools[0]["type"] != tt.wantTool {
				t.Fatalf("tool type = %q, want %q", tools[0]["type"], tt.wantTool)
			}
		})
	}
}

func TestAppendURLCitations(t *testing.T) {
	base := urlAnnotation{Type: "url_citation"}
	base.URLCitation.URL = "https://example.com/page"
	base.URLCitation.Title = "Example"

	other := urlAnnotation{Type: "url_citation"}
	other.URLCitation.URL = "https://example.com/other"

	tests := []struct {
		name        string
		answer      string
		annotations []urlAnnotation
		assert      func(t *testing.T, got string)
	}{
		{
			name:        "deduplicates links",
			answer:      "回答",
			annotations: []urlAnnotation{base, base},
			assert: func(t *testing.T, got string) {
				t.Helper()
				if strings.Count(got, "https://example.com/page") != 1 {
					t.Fatalf("got = %q, want exactly one citation", got)
				}
			},
		},
		{
			name:        "appends citations after answer",
			answer:      "回答",
			annotations: []urlAnnotation{base, other},
			assert: func(t *testing.T, got string) {
				t.Helper()
				if !strings.HasPrefix(got, "回答\n\n网络来源：\n") {
					t.Fatalf("got = %q, want answer followed by citations", got)
				}
				if !strings.Contains(got, "[https://example.com/other](https://example.com/other)") {
					t.Fatalf("got = %q, want fallback title from URL", got)
				}
			},
		},
		{
			name:        "returns citation list when answer empty",
			answer:      "",
			annotations: []urlAnnotation{base},
			assert: func(t *testing.T, got string) {
				t.Helper()
				if got != "网络来源：\n- [Example](https://example.com/page)" {
					t.Fatalf("got = %q", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, appendURLCitations(tt.answer, tt.annotations))
		})
	}
}
