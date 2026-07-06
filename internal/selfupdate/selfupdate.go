package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/version"
)

// ReleaseInfo is the latest release metadata from GitHub.
type ReleaseInfo struct {
	Tag     string `json:"tag_name"`
	URL     string `json:"html_url"`
	Current string
	Behind  bool
}

// CheckLatest compares local version with GitHub releases API.
func CheckLatest(ctx context.Context, repo string) (ReleaseInfo, error) {
	if repo == "" {
		repo = "schmorrison/goshpanel"
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "GoshPanel")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ReleaseInfo{Current: version.Version}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	var raw struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return ReleaseInfo{Current: version.Version}, err
	}
	cur := version.Version
	behind := strings.TrimPrefix(raw.TagName, "v") != strings.TrimPrefix(cur, "v")
	return ReleaseInfo{Tag: raw.TagName, URL: raw.HTMLURL, Current: cur, Behind: behind}, nil
}
