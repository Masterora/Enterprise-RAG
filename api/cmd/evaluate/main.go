package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	var (
		file             string
		baseURL          string
		output           string
		concurrency      int
		timeout          time.Duration
		failOnEvaluation bool
	)
	flag.StringVar(&file, "file", "", "evaluation suite JSON file")
	flag.StringVar(&baseURL, "base-url", "http://localhost:9999", "Enterprise-RAG API base URL")
	flag.StringVar(&output, "output", "", "optional Markdown report path")
	flag.IntVar(&concurrency, "concurrency", 1, "number of concurrent cases")
	flag.DurationVar(&timeout, "timeout", 120*time.Second, "timeout for each case")
	flag.BoolVar(&failOnEvaluation, "fail-on-evaluation", true, "exit unsuccessfully when any case fails evaluation")
	flag.Parse()

	if file == "" {
		exitf("-file is required")
	}
	evaluationSuite, err := loadSuite(file)
	if err != nil {
		exitf("%v", err)
	}
	evaluationRunner, err := newRunner(baseURL, os.Getenv("RAG_API_TOKEN"), concurrency, timeout)
	if err != nil {
		exitf("%v", err)
	}

	results := evaluationRunner.run(context.Background(), evaluationSuite)
	report := renderReport(evaluationSuite, results, time.Now())
	if output == "" {
		fmt.Print(report)
	} else {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			exitf("create report directory: %v", err)
		}
		if err := os.WriteFile(output, []byte(report), 0o644); err != nil {
			exitf("write report: %v", err)
		}
		fmt.Printf("report written to %s\n", output)
	}

	stats := summarize(results)
	if stats.Succeeded != stats.Total || (failOnEvaluation && stats.Passed != stats.Total) {
		os.Exit(1)
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "evaluate: "+format+"\n", args...)
	os.Exit(2)
}
