package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"stage-01/llm"
	"time"
)

func main() {
	mode := flag.String("mode", "mock", "mock or ollama")
	prompt := flag.String("prompt", "请用一句话解释什么是 Agent。", "model prompt")
	flag.Parse()

	endpoint := "http://127.0.0.1:11434/v1"
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		panic("model is required")
	}

	var server *httptest.Server
	if *mode == "mock" {
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"[Mock] Agent 是由模型与受控工具执行流程组成的系统。"},"finish_reason":"stop"}],"usage":{"prompt_tokens":15,"completion_tokens":20,"total_tokens":35}}`)
		}))
		defer server.Close()
		endpoint = server.URL + "/v1"
		model = "mock"
	} else if *mode != "ollama" {
		log.Fatalf("unknown mode: %s", *mode)
	}

	c := llm.Client{HTTP: &http.Client{Timeout: 60 * time.Second}, BaseURL: endpoint}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	started := time.Now()
	resp, err := c.Complete(ctx, llm.Request{Model: model, Messages: []llm.Message{{Role: "user", Content: *prompt}}, MaxTokens: 128})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(resp.Choices[0].Message.Content)
	fmt.Printf("model=%s latency=%s prompt_tokens=%d completion_tokens=%d\n", model, time.Since(started).Round(time.Millisecond), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	if *mode == "mock" {
		fmt.Println("注意：Mock 的 token 和响应内容都是固定测试数据，并非真实模型统计。")
	}
}
