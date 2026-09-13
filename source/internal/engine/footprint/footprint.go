package footprint

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	StatusFound    Status = "FOUND"
	StatusNotFound Status = "NOT_FOUND"
	StatusError    Status = "ERROR"
	StatusUnknown  Status = "UNKNOWN"
)

type Site struct {
	Name             string   `json:"name"`
	URLMain          string   `json:"url_main"`
	ProbeURL         string   `json:"probe_url"`  // contains {username} placeholder
	Method           string   `json:"method"`     // GET or HEAD
	ErrorType        string   `json:"error_type"` // status_code | message | response_url
	ErrorValues      []string `json:"error_values"`
	NotFoundStatuses []int    `json:"not_found_statuses"`
}

type Result struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Status  Status `json:"status"`
	Detail  string `json:"detail,omitempty"`
	Checked bool   `json:"-"`
}

func Probe(ctx context.Context, username string, sites []Site, httpc *http.Client, maxConcurrent int) []Result {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil
	}
	if httpc == nil {
		httpc = &http.Client{Timeout: 20 * time.Second}
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}

	out := make([]Result, 0, len(sites))
	mu := sync.Mutex{}
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, s := range sites {
		site := s
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res := probeOne(ctx, username, site, httpc)
			mu.Lock()
			out = append(out, res)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func probeOne(ctx context.Context, username string, site Site, httpc *http.Client) Result {
	res := Result{Name: site.Name}
	raw := strings.ReplaceAll(site.ProbeURL, "{username}", url.PathEscape(username))
	res.URL = raw
	method := site.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, raw, nil)
	if err != nil {
		res.Status = StatusError
		res.Detail = err.Error()
		return res
	}
	req.Header.Set("User-Agent", "NullTrace/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json;q=0.9,*/*;q=0.8")

	resp, err := httpc.Do(req)
	if err != nil {
		res.Status = StatusError
		res.Detail = err.Error()
		return res
	}
	defer resp.Body.Close()

	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	var body []byte
	if method != http.MethodHead {
		body, _ = io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	}
	low := strings.ToLower(string(body))

	switch site.ErrorType {
	case "message":
		if containsAny(low, site.ErrorValues) {
			res.Status = StatusNotFound
			res.Detail = "error marker present"
		} else {
			res.Status = StatusFound
		}
	case "response_url":
		expected := strings.ToLower(raw)
		final := strings.ToLower(finalURL)
		if containsAny(final, site.ErrorValues) || !strings.Contains(final, strings.ToLower(username)) {
			res.Status = StatusNotFound
			res.Detail = finalURL
		} else if final != "" && final != expected {
			res.Status = StatusFound
			res.Detail = finalURL
		} else {
			res.Status = StatusFound
		}
	default: // status_code
		statuses := site.NotFoundStatuses
		if len(statuses) == 0 {
			statuses = []int{http.StatusNotFound, http.StatusGone}
		}
		if intIn(resp.StatusCode, statuses) {
			res.Status = StatusNotFound
			res.Detail = fmt.Sprintf("http %d", resp.StatusCode)
		} else if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			res.Status = StatusFound
		} else {
			res.Status = StatusError
			res.Detail = fmt.Sprintf("http %d", resp.StatusCode)
		}
	}
	return res
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(haystack, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func intIn(v int, list []int) bool {
	for _, x := range list {
		if v == x {
			return true
		}
	}
	return false
}
