// Package updates checks release metadata on explicit user request. It never
// downloads or executes updates and does not follow redirects.
package updates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Endpoint = "https://api.github.com/repos/Brook-sys/picofarm/releases/latest"

type Release struct {
	Tag        string `json:"tag_name"`
	URL        string `json:"html_url"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// Check retrieves the newest stable release. An empty result means there is no
// published stable release. Version ordering is deliberately left to callers;
// development builds must not be reported as outdated by string comparison.
func Check(ctx context.Context) (*Release, error) {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return check(ctx, client, Endpoint)
}

func check(ctx context.Context, client *http.Client, endpoint string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PicoFarm-desktop-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("release check unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release check returned HTTP %d", resp.StatusCode)
	}
	var release Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&release); err != nil {
		return nil, errors.New("invalid release metadata")
	}
	u, err := url.Parse(release.URL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || !strings.HasPrefix(u.Path, "/Brook-sys/picofarm/releases/tag/") || release.Tag == "" || release.Prerelease || release.Draft {
		return nil, errors.New("untrusted release metadata")
	}
	return &release, nil
}
