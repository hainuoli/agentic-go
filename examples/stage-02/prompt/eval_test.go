package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type mockClassifier struct{}

func (mockClassifier) Classify(_ context.Context, text string) (string, error) {
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

type classifierFunc func(context.Context, string) (string, error)

func (f classifierFunc) Classify(ctx context.Context, text string) (string, error) {
	return f(ctx, text)
}

func TestEvaluate(t *testing.T) {
	t.Parallel()

	cases := []Case{
		{Input: "重复扣款", Want: "billing"},
		{Input: "打开软件后白屏", Want: "bug"},
		{Input: "可以改邮箱吗", Want: "other"},
		{Input: "提交表单后没有反应", Want: "bug"},
	}
	report, err := Evaluate(context.Background(), mockClassifier{}, DefaultSpec(true), cases)
	if err != nil {
		t.Fatal(err)
	}
	if report.Total != 4 || report.Passed != 3 || len(report.Results) != 4 {
		t.Fatalf("report = %+v, want total=4 passed=3 results=4", report)
	}
	if report.Results[3].Got != "other" || report.Results[3].Passed || report.Results[3].Error != "" {
		t.Fatalf("unexpected last result: %+v", report.Results[3])
	}
}

func TestEvaluateErrors(t *testing.T) {
	t.Parallel()

	spec := DefaultSpec(false)
	cases := []Case{{Input: "测试", Want: "other"}}
	if _, err := Evaluate(context.Background(), nil, spec, cases); err == nil {
		t.Error("nil classifier: want error")
	}
	if _, err := Evaluate(context.Background(), mockClassifier{}, spec, nil); err == nil {
		t.Error("empty cases: want error")
	}
	if _, err := Evaluate(context.Background(), mockClassifier{}, spec,
		[]Case{{Input: "测试", Want: "unknown"}}); err == nil {
		t.Error("invalid expected label: want error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Evaluate(ctx, mockClassifier{}, spec, cases); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled context: got %v", err)
	}
}

func TestEvaluateInvalidModelResponse(t *testing.T) {
	t.Parallel()

	model := classifierFunc(func(context.Context, string) (string, error) {
		return "billing because of payment", nil
	})
	report, err := Evaluate(context.Background(), model, DefaultSpec(false),
		[]Case{{Input: "重复扣款", Want: "billing"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed != 0 || report.Results[0].Error != "invalid output label" {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestEvaluateModelError(t *testing.T) {
	t.Parallel()

	model := classifierFunc(func(context.Context, string) (string, error) {
		return "", errors.New("backend unavailable")
	})
	report, err := Evaluate(context.Background(), model, DefaultSpec(false),
		[]Case{{Input: "测试", Want: "other"}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed != 0 || report.Results[0].Error != "backend unavailable" {
		t.Fatalf("unexpected report: %+v", report)
	}
}
