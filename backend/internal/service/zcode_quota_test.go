package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAccountUsesZcodeQuota(t *testing.T) {
	require.False(t, accountUsesZcodeQuota(&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth}))
	require.True(t, accountUsesZcodeQuota(&Account{
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"quota_source": "zcode"},
	}))
	require.True(t, accountUsesZcodeQuota(&Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"c/GLM-5.3": "c/GLM-5.3"},
		},
	}))
	require.False(t, accountUsesZcodeQuota(&Account{
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"model_mapping": map[string]any{"claude-sonnet": "claude-sonnet"}},
	}))
	require.True(t, accountUsesLingxiQuota(&Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://olx.osaka.mangaharb.fun"},
	}))
	require.True(t, accountUsesLingxiQuota(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"plan_pro": "pro"},
		},
	}))
	require.False(t, accountUsesLingxiQuota(&Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://api.openai.com/v1"},
	}))
	require.True(t, accountUsesCodebuddyQuota(&Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://owkbd.orbiqo.cc"},
	}))
	require.False(t, accountUsesCodebuddyQuota(&Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://olx.osaka.mangaharb.fun"},
	}))
}

func TestZcodeQuotaURL(t *testing.T) {
	require.Equal(t, "https://ozc.example/v1/orbiqo/quota", zcodeQuotaURL("https://ozc.example/v1"))
	require.Equal(t, "https://ozc.example/v1/orbiqo/quota", zcodeQuotaURL("https://ozc.example/v1/"))
	require.Equal(t, "https://ozc.example/v1/orbiqo/quota", zcodeQuotaURL("https://ozc.example"))
}

func TestQuarkQuotaAccountAndBearerAuthentication(t *testing.T) {
	for _, credentials := range []map[string]any{
		{"base_url": "https://oqk.orbiqo.cc/v1"},
		{"quota_source": "quark", "base_url": "http://127.0.0.1:8022/v1"},
	} {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: credentials}
		require.True(t, accountUsesOrbiqoQuota(account))
		account.Type = AccountTypeOAuth
		require.False(t, accountUsesOrbiqoQuota(account))
	}
	require.False(t, accountUsesOrbiqoQuota(&Account{
		Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"base_url": "https://example.com", "model_mapping": map[string]any{"qwen3.8-flash": "qwen3.8-flash"}},
	}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/orbiqo/quota", r.URL.Path)
		require.Equal(t, "Bearer quark-local-key", r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, `{"object":"orbiqo.quota","windows":[{"window":"5h","used_percent":0},{"window":"7d","used_percent":63.97,"resets_at":"2026-11-30T03:10:44.034Z"}]}`)
	}))
	t.Cleanup(server.Close)
	svc := &AccountUsageService{cfg: &config.Config{}}
	svc.cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	info, err := svc.fetchZcodeQuota(context.Background(), &Account{
		Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"quota_source": "quark", "api_key": "quark-local-key", "base_url": server.URL},
	})
	require.NoError(t, err)
	require.Equal(t, 0.0, info.FiveHour.Utilization)
	require.Nil(t, info.FiveHour.ResetsAt)
	require.Equal(t, 63.97, info.SevenDay.Utilization)
	require.Equal(t, time.Date(2026, 11, 30, 3, 10, 44, 34000000, time.UTC), *info.SevenDay.ResetsAt)
}

func TestFetchZcodeQuota(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		require.Equal(t, "/v1/orbiqo/quota", r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"object":"orbiqo.quota",
			"plan":"pro",
			"windows":[
				{"window":"5h","used_percent":42,"resets_at":"2026-11-15T10:00:00Z"},
				{"window":"7d","used_percent":8,"resets_at":"2026-11-20T00:00:00Z"}
			],
			"balances":[{"name":"GLM-5.3-Flash","remaining":1500,"total":2000,"used_percent":25}]
		}`)
	}))
	t.Cleanup(server.Close)

	svc := &AccountUsageService{cfg: &config.Config{}}
	svc.cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	account := &Account{
		ID:       5,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":       "local-token",
			"base_url":      server.URL + "/v1",
			"quota_source":  "zcode",
			"model_mapping": map[string]any{"c/GLM-5.3": "c/GLM-5.3"},
		},
	}
	zcodeQuotaCache.Delete(account.ID)

	info, err := svc.getZcodeQuotaUsage(context.Background(), account, true)
	require.NoError(t, err)
	require.Equal(t, "local-token", gotKey)
	require.Equal(t, "pro", info.ZcodePlan)
	require.NotNil(t, info.FiveHour)
	require.Equal(t, 42.0, info.FiveHour.Utilization)
	require.NotNil(t, info.SevenDay)
	require.Equal(t, 8.0, info.SevenDay.Utilization)
	require.Equal(t, time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), info.SevenDay.ResetsAt.UTC())
	require.Len(t, info.ZcodeBalances, 1)
	require.Equal(t, "GLM-5.3-Flash", info.ZcodeBalances[0].Name)
	require.NotNil(t, info.ZcodeBalances[0].Remaining)
	require.Equal(t, 1500.0, *info.ZcodeBalances[0].Remaining)
}
