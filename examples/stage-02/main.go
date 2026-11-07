package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"stage-02/llm"
	"stage-02/prompt"
	"strings"
	"time"
)

type ollamaClassifier struct {
	client llm.Client
	model  string
}

func (c *ollamaClassifier) Classify(ctx context.Context, text string) (string, error) {
	out, err := c.client.Complete(ctx, llm.Request{Model: c.model, MaxTokens: 50, Messages: []llm.Message{{Role: "user", Content: text}}})
	if err != nil {
		return "", err
	}
	return out.Choices[0].Message.Content, nil
}

type MockClassifier struct{}

func (MockClassifier) Classify(_ context.Context, text string) (string, error) {
	i := strings.LastIndex(text, "\nUSER_DATA_JSON: ")
	if i < 0 {
		return "", errors.New("missing input")
	}
	raw := strings.TrimSpace(text[i+len("\nUSER_DATA_JSON: "):])
	var input string
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return "", err
	}
	if strings.Contains(input, "扣款") || strings.Contains(input, "发票") {
		return "billing", nil
	}
	if strings.Contains(input, "白屏") || strings.Contains(input, "闪退") {
		return "bug", nil
	}
	return "other", nil
}

func main() {
	mode := flag.String("mode", "mock", "mock or ollama")
	casesPath := flag.String("cases", "testdata/cases.json", "case file path from repo root")
	flag.Parse()

	raw, err := os.ReadFile(*casesPath)
	if err != nil {
		log.Fatal(err)
	}
	var cases []prompt.Case
	if err := json.Unmarshal(raw, &cases); err != nil {
		log.Fatal(err)
	}
	var grader prompt.Classifier = MockClassifier{}
	if *mode == "ollama" {
		model := os.Getenv("OLLAMA_MODEL")
		if model == "" {
			panic("model is required")
		}
		grader = &ollamaClassifier{client: llm.Client{HTTP: &http.Client{Timeout: 60 * time.Second}, BaseURL: "http://127.0.0.1:11434/v1"}, model: model}
	} else if *mode != "mock" {
		log.Fatalf("unknown mode: %s", *mode)
	}

	for _, fewShot := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
		report, err := prompt.Evaluate(ctx, grader, prompt.DefaultSpec(fewShot), cases)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		name := "zero-shot"
		if fewShot {
			name = "three-shot"
		}
		fmt.Printf("%s: %d/%d\n", name, report.Passed, report.Total)
		for _, r := range report.Results {
			fmt.Printf("  %-22s expected=%s got=%s error=%s\n", r.Case.Input, r.Case.Want, r.Got, r.Error)
		}
	}
	if *mode == "mock" {
		fmt.Println("注意：mock 为固定规则分类器，两个分数不能证明 Few-Shot 是否改善真实模型。")
	}
}
