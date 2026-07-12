package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"enterprise-rag/backend/internal/types"
)

type CompatibleClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

func NewCompatibleClient(apiKey, model, baseURL string) *CompatibleClient {
	return &CompatibleClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *CompatibleClient) Generate(ctx context.Context, prompt string, webSearch bool) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("llm api key is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return "", errors.New("llm model is required")
	}
	if c.baseURL == "" {
		return "", errors.New("llm base url is required")
	}

	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":      800,
		"enable_thinking": false,
	}
	if err := c.enableWebSearch(payload, webSearch); err != nil {
		return "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm compatible chat request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content     string          `json:"content"`
				Annotations []urlAnnotation `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("llm response choices is empty")
	}

	answer := strings.TrimSpace(parsed.Choices[0].Message.Content)
	answer = appendURLCitations(answer, parsed.Choices[0].Message.Annotations)
	if answer == "" {
		return "", errors.New("llm response text is empty")
	}
	return answer, nil
}

func (c *CompatibleClient) SearchWeb(ctx context.Context, query string) ([]types.ExternalLink, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": buildWebSearchPrompt(query),
			},
		},
		"max_tokens":      1000,
		"enable_thinking": false,
	}
	if err := c.enableWebSearch(payload, true); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm compatible web search request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content     string          `json:"content"`
				Annotations []urlAnnotation `json:"annotations"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("llm web search response choices is empty")
	}

	return collectExternalLinks(parsed.Choices[0].Message.Content, parsed.Choices[0].Message.Annotations), nil
}

func (c *CompatibleClient) GenerateStream(ctx context.Context, prompt string, webSearch bool, onDelta func(string) error) error {
	if err := c.validate(); err != nil {
		return err
	}

	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens":      800,
		"stream":          true,
		"enable_thinking": false,
	}
	if err := c.enableWebSearch(payload, webSearch); err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		return fmt.Errorf("llm compatible stream request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	annotations := make([]urlAnnotation, 0)
	flushAnnotations := func() error {
		citations := appendURLCitations("", annotations)
		if citations == "" {
			return nil
		}
		return onDelta("\n\n" + citations)
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return flushAnnotations()
		}

		var parsed struct {
			Choices []struct {
				Delta struct {
					Content     string          `json:"content"`
					Annotations []urlAnnotation `json:"annotations"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return err
		}
		shouldStop := false
		for _, choice := range parsed.Choices {
			annotations = append(annotations, choice.Delta.Annotations...)
			if choice.Delta.Content == "" {
				if choice.FinishReason != "" {
					shouldStop = true
				}
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return err
			}
			if choice.FinishReason != "" {
				shouldStop = true
			}
		}
		if shouldStop {
			return flushAnnotations()
		}
	}
	return scanner.Err()
}

type urlAnnotation struct {
	Type        string `json:"type"`
	URLCitation struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"url_citation"`
}

func appendURLCitations(answer string, annotations []urlAnnotation) string {
	seen := make(map[string]struct{})
	links := make([]string, 0, len(annotations))
	for _, annotation := range annotations {
		url := strings.TrimSpace(annotation.URLCitation.URL)
		if annotation.Type != "url_citation" || url == "" {
			continue
		}
		if _, exists := seen[url]; exists {
			continue
		}
		seen[url] = struct{}{}
		title := strings.TrimSpace(annotation.URLCitation.Title)
		if title == "" {
			title = url
		}
		links = append(links, fmt.Sprintf("- [%s](%s)", title, url))
	}
	if len(links) == 0 {
		return strings.TrimSpace(answer)
	}
	if strings.TrimSpace(answer) == "" {
		return "网络来源：\n" + strings.Join(links, "\n")
	}
	return strings.TrimSpace(answer) + "\n\n网络来源：\n" + strings.Join(links, "\n")
}

func (c *CompatibleClient) enableWebSearch(payload map[string]any, enabled bool) error {
	if !enabled {
		return nil
	}
	if !strings.Contains(strings.ToLower(c.baseURL), "openrouter.ai") {
		return errors.New("web search is only supported by the configured OpenRouter provider")
	}
	payload["tools"] = []map[string]string{{"type": "openrouter:web_search"}}
	return nil
}

func (c *CompatibleClient) validate() error {
	if c.apiKey == "" {
		return errors.New("llm api key is required")
	}
	if strings.TrimSpace(c.model) == "" {
		return errors.New("llm model is required")
	}
	if c.baseURL == "" {
		return errors.New("llm base url is required")
	}
	return nil
}

func buildWebSearchPrompt(query string) string {
	return "你是联网搜索助手。必须调用网络搜索工具，查找与问题最直接相关的公开网页资料。\n" +
		"返回要求：\n" +
		"1. 只返回 JSON 数组。\n" +
		"2. 每一项必须包含 title、url、snippet 三个字段。\n" +
		"3. 最多返回 5 条。\n" +
		"4. 如果没有找到可靠公开资料，返回 []。\n" +
		"5. 不要输出 Markdown，不要输出解释文字。\n\n" +
		"问题：\n" + strings.TrimSpace(query)
}

func collectExternalLinks(content string, annotations []urlAnnotation) []types.ExternalLink {
	links := make([]types.ExternalLink, 0)
	seen := make(map[string]struct{})

	appendLink := func(link types.ExternalLink) {
		link.URL = strings.TrimSpace(link.URL)
		if !isUsableExternalURL(link.URL) {
			return
		}
		if _, exists := seen[link.URL]; exists {
			return
		}
		link.Title = strings.TrimSpace(link.Title)
		link.Snippet = strings.TrimSpace(link.Snippet)
		if link.Title == "" {
			link.Title = link.URL
		}
		seen[link.URL] = struct{}{}
		links = append(links, link)
	}

	for _, link := range parseExternalLinksJSON(content) {
		appendLink(link)
	}
	for _, annotation := range annotations {
		if annotation.Type != "url_citation" {
			continue
		}
		appendLink(types.ExternalLink{
			Title: annotation.URLCitation.Title,
			URL:   annotation.URLCitation.URL,
		})
	}
	for _, url := range extractRawURLs(content) {
		appendLink(types.ExternalLink{Title: url, URL: url})
	}

	if len(links) > 5 {
		return links[:5]
	}
	return links
}

func parseExternalLinksJSON(content string) []types.ExternalLink {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var direct []types.ExternalLink
	if err := json.Unmarshal([]byte(content), &direct); err == nil {
		return direct
	}

	var wrapped struct {
		Links []types.ExternalLink `json:"links"`
	}
	if err := json.Unmarshal([]byte(content), &wrapped); err == nil {
		return wrapped.Links
	}

	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		var partial []types.ExternalLink
		if err := json.Unmarshal([]byte(content[start:end+1]), &partial); err == nil {
			return partial
		}
	}
	return nil
}

func extractRawURLs(content string) []string {
	pattern := regexp.MustCompile(`https?://[^\s\])"]+`)
	matches := pattern.FindAllString(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, exists := seen[match]; exists {
			continue
		}
		seen[match] = struct{}{}
		urls = append(urls, match)
	}
	return urls
}

func isUsableExternalURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" || host == "www" || !strings.Contains(host, ".") {
		return false
	}
	parts := strings.Split(host, ".")
	last := parts[len(parts)-1]
	if len(last) < 2 {
		return false
	}
	return true
}
