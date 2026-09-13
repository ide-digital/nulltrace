package captcha

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	key   string
	base  string
	httpc *http.Client
}

func New(apiKey, base string, httpc *http.Client) *Client {
	if base == "" {
		base = "https://api.capsolver.com"
	}
	if httpc == nil {
		httpc = &http.Client{Timeout: 90 * time.Second}
	}
	return &Client{key: apiKey, base: strings.TrimRight(base, "/"), httpc: httpc}
}

type createTaskReq struct {
	ClientKey string   `json:"clientKey"`
	Task      taskBody `json:"task"`
}

type taskBody struct {
	Type       string `json:"type"`
	WebsiteURL string `json:"websiteURL"`
	WebsiteKey string `json:"websiteKey"`
}

type createTaskResp struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	TaskID           string `json:"taskId"`
}

type getTaskReq struct {
	ClientKey string `json:"clientKey"`
	TaskID    string `json:"taskId"`
}

type getTaskResp struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	Status           string `json:"status"`
	Solution         struct {
		GRecaptchaResponse string `json:"gRecaptchaResponse"`
		CaptchaKey         string `json:"captchaKey"`
		UserAgent          string `json:"userAgent"`
	} `json:"solution"`
}

func (c *Client) SolveRecaptchaV2(ctx context.Context, websiteURL, siteKey string) (string, error) {
	return c.solve(ctx, "ReCaptchaV2TaskProxyLess", websiteURL, siteKey)
}

func (c *Client) SolveHCaptcha(ctx context.Context, websiteURL, siteKey string) (string, error) {
	return c.solve(ctx, "HCaptchaTaskProxyLess", websiteURL, siteKey)
}

func (c *Client) solve(ctx context.Context, typ, websiteURL, siteKey string) (string, error) {
	if c == nil || c.key == "" {
		return "", fmt.Errorf("capsolver: no api key configured")
	}
	create := createTaskReq{ClientKey: c.key, Task: taskBody{Type: typ, WebsiteURL: websiteURL, WebsiteKey: siteKey}}
	var cr createTaskResp
	if err := c.post(ctx, "/createTask", create, &cr); err != nil {
		return "", err
	}
	if cr.ErrorID != 0 {
		return "", fmt.Errorf("capsolver createTask: %s (%s)", cr.ErrorCode, cr.ErrorDescription)
	}
	if cr.TaskID == "" {
		return "", fmt.Errorf("capsolver: empty task id")
	}

	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
			var gr getTaskResp
			if err := c.post(ctx, "/getTaskResult", getTaskReq{ClientKey: c.key, TaskID: cr.TaskID}, &gr); err != nil {
				return "", err
			}
			if gr.ErrorID != 0 {
				return "", fmt.Errorf("capsolver getTaskResult: %s", gr.ErrorCode)
			}
			switch gr.Status {
			case "processing":
				continue
			case "ready":
				tok := gr.Solution.GRecaptchaResponse
				if tok == "" {
					tok = gr.Solution.CaptchaKey
				}
				if tok == "" {
					return "", fmt.Errorf("capsolver: empty solution token")
				}
				return tok, nil
			default:
				return "", fmt.Errorf("capsolver: unknown status %q", gr.Status)
			}
		}
	}
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NullTrace/1.0")
	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("capsolver %s: %w", path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("capsolver %s: http %d: %s", path, resp.StatusCode, clip(string(data), 200))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("capsolver %s decode: %w", path, err)
	}
	return nil
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
