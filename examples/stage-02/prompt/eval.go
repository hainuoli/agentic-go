package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type (
	Case struct {
		Input string `json:"input"`
		Want  string `json:"want"`
	}

	Result struct {
		Case   Case
		Got    string
		Passed bool
		Error  string
	}

	Report struct {
		Total   int
		Passed  int
		Results []Result
	}

	Classifier interface {
		Classify(context.Context, string) (string, error)
	}
)

func Evaluate(ctx context.Context, model Classifier, spec Spec, cases []Case) (Report, error) {
	if model == nil {
		return Report{}, errors.New("nil classifier")
	}
	if len(cases) == 0 {
		return Report{}, errors.New("empty eval cases")
	}
	res := Report{Total: len(cases)}
	for _, c := range cases {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if c.Want != "billing" && c.Want != "bug" && c.Want != "other" {
			return res, fmt.Errorf("invalid expected label %q", c.Want)
		}
		built, err := spec.Build(c.Input)
		if err != nil {
			return res, err
		}
		got, err := model.Classify(ctx, built)
		line := Result{Case: c, Got: strings.TrimSpace(got)}
		if err != nil {
			line.Error = err.Error()
		} else if line.Got != "billing" && line.Got != "bug" && line.Got != "other" {
			line.Error = "invalid output label"
		} else if line.Got == c.Want {
			line.Passed = true
			res.Passed++
		}
		res.Results = append(res.Results, line)
	}
	return res, nil
}
