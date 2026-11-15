package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const zcodeQuotaCacheTTL = time.Minute

type ZcodeQuotaBalance struct {
	Name        string     `json:"name"`
	Remaining   *float64   `json:"remaining,omitempty"`
	Total       *float64   `json:"total,omitempty"`
	UsedPercent *float64   `json:"used_percent,omitempty"`
	ResetsAt    *time.Time `json:"resets_at,omitempty"`
}

type zcodeQuotaCacheEntry struct {
	info *UsageInfo
	at   time.Time
}

var (
	zcodeQuotaCache sync.Map
	zcodeQuotaHTTP  = &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

func accountUsesZcodeQuota(account *Account) bool {
	if account == nil || account.Platform != PlatformAnthropic || account.Type != AccountTypeAPIKey {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(account.GetCredential("quota_source")), "zcode") {
		return true
	}
	mapping, _ := account.Credentials["model_mapping"].(map[string]any)
	for key, value := range mapping {
		if zcodeModelAlias(key) {
			return true
		}
		if text, ok := value.(string); ok && zcodeModelAlias(text) {
			return true
		}
	}
	return false
}

func zcodeModelAlias(value string) bool {
	return strings.HasPrefix(value, "c/") || strings.HasPrefix(value, "s/")
}

func zcodeQuotaURL(baseURL string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		return base + "/orbiqo/quota"
	}
	return base + "/v1/orbiqo/quota"
}

func (s *AccountUsageService) getZcodeQuotaUsage(ctx context.Context, account *Account, force bool) (*UsageInfo, error) {
	if !force {
		if cached, ok := zcodeQuotaCache.Load(account.ID); ok {
			if entry, ok := cached.(zcodeQuotaCacheEntry); ok && time.Since(entry.at) < zcodeQuotaCacheTTL && entry.info != nil {
				refreshZcodeRemaining(entry.info, time.Now())
				return entry.info, nil
			}
		}
	}

	info, err := s.fetchZcodeQuota(ctx, account)
	if err != nil {
		return nil, err
	}
	zcodeQuotaCache.Store(account.ID, zcodeQuotaCacheEntry{info: info, at: time.Now()})
	return info, nil
}

func (s *AccountUsageService) fetchZcodeQuota(ctx context.Context, account *Account) (*UsageInfo, error) {
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, fmt.Errorf("zcode quota api_key is empty")
	}
	baseURL := strings.TrimSpace(account.GetCredential("base_url"))
	if baseURL == "" {
		baseURL = account.GetBaseURL()
	}
	target := zcodeQuotaURL(baseURL)
	validated, err := cnValidateProbeURL(s.cfg, target)
	if err != nil {
		return nil, fmt.Errorf("zcode quota url rejected: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, validated, nil)
	if err != nil {
		return nil, fmt.Errorf("zcode quota request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := zcodeQuotaHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("zcode quota request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("zcode quota read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("zcode quota http %d", resp.StatusCode)
	}
	info, err := parseZcodeQuota(body)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	info.Source = "active"
	info.UpdatedAt = &now
	refreshZcodeRemaining(info, now)
	return info, nil
}

func parseZcodeQuota(body []byte) (*UsageInfo, error) {
	var payload struct {
		Object  string `json:"object"`
		Plan    string `json:"plan"`
		Error   string `json:"error"`
		Windows []struct {
			Window      string  `json:"window"`
			UsedPercent float64 `json:"used_percent"`
			ResetsAt    *string `json:"resets_at"`
		} `json:"windows"`
		Balances []struct {
			Name        string   `json:"name"`
			Remaining   *float64 `json:"remaining"`
			Total       *float64 `json:"total"`
			UsedPercent *float64 `json:"used_percent"`
			ResetsAt    *string  `json:"resets_at"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("zcode quota decode: %w", err)
	}
	if payload.Object != "orbiqo.quota" {
		return nil, fmt.Errorf("zcode quota response was not recognized")
	}
	info := &UsageInfo{ZcodePlan: strings.TrimSpace(payload.Plan), Error: strings.TrimSpace(payload.Error)}
	for _, window := range payload.Windows {
		progress := &UsageProgress{Utilization: window.UsedPercent, ResetsAt: parseZcodeTime(window.ResetsAt)}
		switch window.Window {
		case "5h":
			if info.FiveHour == nil {
				info.FiveHour = progress
			}
		case "7d":
			if info.SevenDay == nil {
				info.SevenDay = progress
			}
		}
	}
	for _, balance := range payload.Balances {
		name := strings.TrimSpace(balance.Name)
		if name == "" {
			continue
		}
		info.ZcodeBalances = append(info.ZcodeBalances, ZcodeQuotaBalance{
			Name:        name,
			Remaining:   balance.Remaining,
			Total:       balance.Total,
			UsedPercent: balance.UsedPercent,
			ResetsAt:    parseZcodeTime(balance.ResetsAt),
		})
	}
	return info, nil
}

func parseZcodeTime(value *string) *time.Time {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil
	}
	return &parsed
}

func refreshZcodeRemaining(info *UsageInfo, now time.Time) {
	if info == nil {
		return
	}
	for _, window := range []*UsageProgress{info.FiveHour, info.SevenDay} {
		if window == nil || window.ResetsAt == nil {
			continue
		}
		remaining := int(window.ResetsAt.Sub(now).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		window.RemainingSeconds = remaining
	}
}
