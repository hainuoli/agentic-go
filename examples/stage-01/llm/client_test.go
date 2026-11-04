package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestComplete(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path/method %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing Authorization")
		}
		var in Request
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Error(err)
		}
		if in.Model != "mock-model" || in.Messages[0].Content != "hello" {
			t.Errorf("input %+v", in)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"你好"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`))
	}))
	defer ts.Close()
	c := Client{HTTP: ts.Client(), BaseURL: ts.URL + "/v1", APIKey: "test-key"}
	out, err := c.Complete(context.Background(), Request{Model: "mock-model", Messages: []Message{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Choices[0].Message.Content != "你好" || out.Usage.CompletionTokens != 3 {
		t.Fatalf("got %+v", out)
	}
}

func TestFailures(t *testing.T) {
	cases := []struct {
		name string
		code int
		text string
	}{
		{"http error", 429, `{}`},
		{"bad json", 200, `oops`},
		{"empty", 200, `{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`},
		{"truncated", 200, `{"choices":[{"message":{"role":"assistant","content":"part"},"finish_reason":"length"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.code); _, _ = w.Write([]byte(tc.text)) }))
			defer ts.Close()
			c := Client{HTTP: ts.Client(), BaseURL: ts.URL}
			_, err := c.Complete(context.Background(), Request{Model: "mock", Messages: []Message{{Role: "user", Content: "hi"}}})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTimeoutAndCost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()
	c := Client{HTTP: &http.Client{Timeout: 10 * time.Millisecond}, BaseURL: ts.URL}
	_, err := c.Complete(context.Background(), Request{Model: "mock", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatal("expected timeout")
	}
	price, err := EstimateCostUSD(Usage{PromptTokens: 2000, CompletionTokens: 1000}, Rates{InputPerMillion: 1, OutputPerMillion: 5})
	if err != nil || price != 0.007 {
		t.Fatalf("cost=%f err=%v", price, err)
	}
	_, err = EstimateCostUSD(Usage{PromptTokens: -1}, Rates{})
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("expected negative token error, %v", err)
	}
}
