package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxResponseBytes = 1 << 20

var githubLogin = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

type Profile struct {
	Login       string `json:"login"`
	Name        string `json:"name"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
}

// FetchProfile calls a GitHub-compatible endpoint. The baseURL is injected for
// httptest; real programs should not let an untrusted caller choose this URL.
func FetchProfile(ctx context.Context, client *http.Client, baseURL, username string) (Profile, error) {
	if !githubLogin.MatchString(username) || strings.Contains(username, "--") {
		return Profile{}, errors.New("invalid GitHub username")
	}
	if client == nil {
		return Profile{}, errors.New("nil http client")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/users/" + url.PathEscape(username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Profile{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "agentic-go-foundations-study")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return Profile{}, fmt.Errorf("request profile: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Profile{}, fmt.Errorf("GitHub API status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return Profile{}, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return Profile{}, errors.New("profile response too large")
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("decode profile: %w", err)
	}
	if profile.Login == "" || profile.Followers < 0 || profile.PublicRepos < 0 {
		return Profile{}, errors.New("profile contains invalid data")
	}
	return profile, nil
}

func Summary(p Profile) string {
	return fmt.Sprintf("%s 有 %d 位关注者，%d 个公开仓库", p.Login, p.Followers, p.PublicRepos)
}

// WriteSummary writes public information only. No API keys are ever serialized.
func WriteSummary(filename, value string) error {
	dir := filepath.Dir(filename)
	temp, err := os.CreateTemp(dir, ".summary-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := fmt.Fprintln(temp, value); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, filename)
}
