package recon

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/engine/mailer"
)

const hibpUA = "NullTrace/1.0"

type Finding struct {
	BrokerID   string  `json:"broker_id"`
	BrokerName string  `json:"broker_name"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
	RiskTier   string  `json:"risk_tier"`
	ProfileURL string  `json:"profile_url,omitempty"`
	Details    any     `json:"details,omitempty"`
}

type BreachHit struct {
	Name        string   `json:"name"`
	Domain      string   `json:"domain"`
	BreachDate  string   `json:"breach_date"`
	DataClasses []string `json:"data_classes"`
	Email       string   `json:"email"`
}

type Report struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Findings    []Finding   `json:"findings"`
	Breaches    []BreachHit `json:"breaches"`
}

type Coordinator struct {
	HTTP       *http.Client
	Registry   *broker.Registry
	HIBPBase   string
	HIBPKey    []byte
	EnableHIBP bool
}

func New(reg *broker.Registry, hibpBase string, hibpKey []byte, enable bool) *Coordinator {
	if hibpBase == "" {
		hibpBase = "https://haveibeenpwned.com/api/v3"
	}
	return &Coordinator{
		HTTP: &http.Client{
			Timeout: 20 * time.Second,
		},
		Registry:   reg,
		HIBPBase:   strings.TrimRight(hibpBase, "/"),
		HIBPKey:    hibpKey,
		EnableHIBP: enable,
	}
}

func (c *Coordinator) Scan(ctx context.Context, id mailer.IdentityView) (Report, error) {
	rep := Report{GeneratedAt: time.Now().UTC()}
	if c.Registry != nil {
		rep.Findings = append(rep.Findings, MapBrokers(id, c.Registry.All())...)
	}
	if c.EnableHIBP && len(c.HIBPKey) > 0 {
		hits, err := c.CheckBreaches(ctx, id.Emails)
		if err != nil {
			return rep, err
		}
		rep.Breaches = hits
		for _, h := range hits {
			rep.Findings = append(rep.Findings, Finding{
				BrokerID:   "hibp-" + slug(h.Name),
				BrokerName: "Have I Been Pwned: " + h.Name,
				Reason:     fmt.Sprintf("email %s listed in breach %s", h.Email, h.Name),
				Confidence: 0.92,
				RiskTier:   breachTier(h.DataClasses),
				Details:    h,
			})
		}
	}
	return rep, ctx.Err()
}

func slug(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	return strings.Trim(s, "-")
}

func breachTier(classes []string) string {
	for _, c := range classes {
		low := strings.ToLower(c)
		if strings.Contains(low, "password") || strings.Contains(low, "social security") || strings.Contains(low, "credit") {
			return "CRITICAL"
		}
	}
	for _, c := range classes {
		low := strings.ToLower(c)
		if strings.Contains(low, "phone") || strings.Contains(low, "address") {
			return "HIGH"
		}
	}
	return "MEDIUM"
}

type hibpBreach struct {
	Name        string   `json:"Name"`
	Domain      string   `json:"Domain"`
	BreachDate  string   `json:"BreachDate"`
	DataClasses []string `json:"DataClasses"`
}

func (c *Coordinator) CheckBreaches(ctx context.Context, emails []string) ([]BreachHit, error) {
	var (
		mu   sync.Mutex
		hits []BreachHit
		errs []error
	)
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	for _, email := range emails {
		email := strings.TrimSpace(email)
		if email == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			h, err := c.oneEmail(ctx, email)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			hits = append(hits, h...)
		}()
	}
	wg.Wait()
	if len(errs) > 0 {
		return hits, fmt.Errorf("hibp: %w", errs[0])
	}
	return hits, nil
}

func (c *Coordinator) oneEmail(ctx context.Context, email string) ([]BreachHit, error) {
	u := c.HIBPBase + "/breachedaccount/" + url.PathEscape(email) + "?truncateResponse=false"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("hibp-api-key", string(c.HIBPKey))
	req.Header.Set("User-Agent", hibpUA)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("hibp rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hibp status %d: %s", resp.StatusCode, clip(string(body), 180))
	}
	var parsed []hibpBreach
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("hibp decode: %w", err)
	}
	out := make([]BreachHit, 0, len(parsed))
	for _, p := range parsed {
		out = append(out, BreachHit{
			Name:        p.Name,
			Domain:      p.Domain,
			BreachDate:  p.BreachDate,
			DataClasses: p.DataClasses,
			Email:       email,
		})
	}
	return out, nil
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// PwnedPasswordPrefix queries the k-anonymity Pwned Passwords range API.
// Only the first 5 hex chars of SHA-1(password) leave the machine.
func PwnedPasswordPrefix(ctx context.Context, httpc *http.Client, password []byte) (count int, err error) {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	sum := sha1.Sum(password)
	hexed := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := hexed[:5], hexed[5:]
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.pwnedpasswords.com/range/"+prefix, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", hibpUA)
	req.Header.Set("Add-Padding", "true")
	resp, err := httpc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(parts[0], suffix) {
			var n int
			_, _ = fmt.Sscanf(parts[1], "%d", &n)
			return n, nil
		}
	}
	return 0, nil
}
