package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	modeRetrieval = "retrieval"
	modeAnswer    = "answer"
)

type suite struct {
	Name        string     `json:"name"`
	Mode        string     `json:"mode"`
	SubjectID   string     `json:"subject_id"`
	TopK        int        `json:"top_k"`
	LLMProvider string     `json:"llm_provider"`
	LLMModel    string     `json:"llm_model"`
	Cases       []testCase `json:"cases"`
}

type testCase struct {
	Name             string   `json:"name"`
	Query            string   `json:"query"`
	ExpectedDocIDs   []string `json:"expected_doc_ids"`
	ExpectedChunkIDs []string `json:"expected_chunk_ids"`
	ExpectedRoute    string   `json:"expected_route"`
	ExpectedOutcome  string   `json:"expected_outcome"`
}

func loadSuite(path string) (suite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return suite{}, fmt.Errorf("read evaluation suite: %w", err)
	}

	var result suite
	if err := json.Unmarshal(data, &result); err != nil {
		return suite{}, fmt.Errorf("decode evaluation suite: %w", err)
	}
	if err := result.validate(); err != nil {
		return suite{}, err
	}
	return result, nil
}

func (s *suite) validate() error {
	s.Name = strings.TrimSpace(s.Name)
	s.Mode = strings.ToLower(strings.TrimSpace(s.Mode))
	s.SubjectID = strings.TrimSpace(s.SubjectID)
	if s.Mode == "" {
		s.Mode = modeRetrieval
	}
	if s.Mode != modeRetrieval && s.Mode != modeAnswer {
		return errors.New("mode must be retrieval or answer")
	}
	if s.SubjectID == "" {
		return errors.New("subject_id is required")
	}
	if s.TopK < 0 {
		return errors.New("top_k must not be negative")
	}
	if len(s.Cases) == 0 {
		return errors.New("at least one evaluation case is required")
	}
	for i := range s.Cases {
		s.Cases[i].Name = strings.TrimSpace(s.Cases[i].Name)
		s.Cases[i].Query = strings.TrimSpace(s.Cases[i].Query)
		if s.Cases[i].Query == "" {
			return fmt.Errorf("cases[%d].query is required", i)
		}
		if s.Cases[i].Name == "" {
			s.Cases[i].Name = fmt.Sprintf("case-%d", i+1)
		}
	}
	return nil
}
