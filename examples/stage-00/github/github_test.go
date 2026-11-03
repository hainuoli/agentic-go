package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchProfile(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/gopher" {
			t.Errorf("path %q", r.URL.Path)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"gopher","followers":12,"public_repos":5}`))
	}))
	defer ts.Close()
	got, err := FetchProfile(context.Background(), ts.Client(), ts.URL, "gopher")
	if err != nil {
		t.Fatal(err)
	}
	if got.Login != "gopher" || got.Followers != 12 || got.PublicRepos != 5 {
		t.Fatalf("got %+v", got)
	}
}

func TestFetchProfileFailures(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		body    string
		errPart string
	}{
		{"HTTP error", 429, `{}`, "status"},
		{"invalid JSON", 200, `{oops`, "decode"},
		{"empty login", 200, `{"followers":1}`, "invalid data"},
		{"huge body", 200, strings.Repeat("x", maxResponseBytes+1), "too large"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()
			_, err := FetchProfile(context.Background(), ts.Client(), ts.URL, "gopher")
			if err == nil || !strings.Contains(err.Error(), tc.errPart) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestFetchProfileInvalidInputAndCancel(t *testing.T) {
	for _, login := range []string{"", "-bad", "bad-", "../admin", "x y", strings.Repeat("a", 40)} {
		if _, err := FetchProfile(context.Background(), http.DefaultClient, "http://127.0.0.1", login); err == nil {
			t.Errorf("accepted %q", login)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	if _, err := FetchProfile(ctx, client, "https://api.github.com", "gopher"); err == nil {
		t.Fatal("expected cancel")
	}
}

func TestWriteSummary(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "result.txt")
	if err := WriteSummary(filename, "hello"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filename)
	if err != nil || string(got) != "hello\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}
