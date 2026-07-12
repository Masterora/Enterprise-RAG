package llm

import (
	"testing"

	"enterprise-rag/backend/internal/types"
)

func TestCollectExternalLinks(t *testing.T) {
	annotations := []urlAnnotation{
		{
			Type: "url_citation",
			URLCitation: struct {
				URL   string `json:"url"`
				Title string `json:"title"`
			}{
				URL:   "https://example.com/from-annotation",
				Title: "From Annotation",
			},
		},
	}

	got := collectExternalLinks(`[{"title":"From JSON","url":"https://example.com/from-json","snippet":"json snippet"}]`, annotations)
	want := []types.ExternalLink{
		{Title: "From JSON", URL: "https://example.com/from-json", Snippet: "json snippet"},
		{Title: "From Annotation", URL: "https://example.com/from-annotation", Snippet: ""},
	}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("item %d = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func TestIsUsableExternalURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{url: "https://www", want: false},
		{url: "https://example.com", want: true},
		{url: "http://docs.openai.com", want: true},
		{url: "ftp://example.com", want: false},
		{url: "https://localhost", want: false},
	}

	for _, tc := range cases {
		if got := isUsableExternalURL(tc.url); got != tc.want {
			t.Fatalf("url %q => %v, want %v", tc.url, got, tc.want)
		}
	}
}
