package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelsExposeRoutedTokenLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := func(id int64, window, output int64) service.Account {
		a := service.Account{ID: id, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"public-model": "upstream-model", "unknown": "unknown"}}}
		if window > 0 {
			a.SetUpstreamModelMetadataSnapshot(service.UpstreamModelMetadataSnapshot{Models: map[string]service.UpstreamModelMetadata{
				"upstream-model": {ContextWindow: window, MaxContextWindow: window, MaxInputTokens: window, MaxOutputTokens: output},
			}})
		}
		return a
	}
	for _, tc := range []struct {
		name           string
		accounts       []service.Account
		window, output float64
	}{
		{"single", []service.Account{account(1, 1000000, 64000)}, 1000000, 64000},
		{"shared lower bound", []service.Account{account(1, 1000000, 64000), account(2, 300000, 32000)}, 300000, 32000},
		{"unknown peer", []service.Account{account(1, 1000000, 64000), account(2, 0, 0)}, 0, 0},
		{"unknown output", []service.Account{account(1, 1000000, 64000), account(2, 1000000, 0)}, 1000000, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{ID: 73, Platform: service.PlatformOpenAI}
			h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{group.ID: tc.accounts}})
			list := requestModelForTest(h, group, "", "")
			require.Equal(t, http.StatusOK, list.Code)
			var catalog struct {
				Data []map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(list.Body.Bytes(), &catalog))
			for _, model := range catalog.Data {
				if model["id"] != "public-model" || tc.window == 0 {
					require.NotContains(t, model, "context_window")
					require.NotContains(t, model, "max_output_tokens")
					continue
				}
				require.Equal(t, tc.window, model["context_window"])
				require.Equal(t, tc.window, model["max_context_window"])
				require.Equal(t, tc.window, model["max_input_tokens"])
				if tc.output > 0 {
					require.Equal(t, tc.output, model["max_output_tokens"])
					require.Equal(t, tc.output, model["max_tokens"])
				} else {
					require.NotContains(t, model, "max_output_tokens")
				}
				retrieved := requestModelForTest(h, group, "public-model", "")
				encoded, err := json.Marshal(model)
				require.NoError(t, err)
				require.JSONEq(t, string(encoded), retrieved.Body.String())
			}
			if tc.name == "shared lower bound" {
				group.ModelRoutingEnabled = true
				group.ModelRouting = map[string][]int64{"public-model": {1}}
				var routed map[string]any
				require.NoError(t, json.Unmarshal(requestModelForTest(h, group, "public-model", "").Body.Bytes(), &routed))
				require.Equal(t, float64(1000000), routed["context_window"])
				require.Equal(t, float64(64000), routed["max_output_tokens"])
			}
			group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"unknown"}}
			require.Equal(t, http.StatusNotFound, requestModelForTest(h, group, "public-model", "").Code)
		})
	}
}

func TestModelsTokenLimitsIncludeAllConcretePlatforms(t *testing.T) {
	for _, platform := range []string{service.PlatformCommandCode, service.PlatformCline} {
		group := &service.Group{ID: 75, Platform: platform}
		account := service.Account{ID: 1, Platform: platform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
			Credentials: map[string]any{"model_mapping": map[string]any{"custom": "custom"}}}
		account.SetUpstreamModelMetadataSnapshot(service.UpstreamModelMetadataSnapshot{Models: map[string]service.UpstreamModelMetadata{"custom": {ContextWindow: 1000000, MaxOutputTokens: 64000}}})
		h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{group.ID: {account}}})
		rec := requestModelForTest(h, group, "custom", "")
		require.Equal(t, http.StatusOK, rec.Code, platform)
		var model map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &model))
		require.Equal(t, float64(1000000), model["context_window"], platform)
	}
}

func TestModelsCompositeTokenLimitsAcrossEndpoints(t *testing.T) {
	group := &service.Group{ID: 74, Platform: service.PlatformComposite}
	account := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"large": "large", "small": "small"}}}
	account.SetUpstreamModelMetadataSnapshot(service.UpstreamModelMetadataSnapshot{Models: map[string]service.UpstreamModelMetadata{
		"large": {ContextWindow: 1000000, MaxOutputTokens: 64000},
		"small": {ContextWindow: 300000, MaxOutputTokens: 32000},
	}})
	routes := &gatewayModelsRouteRepoStub{routes: []service.CompositeModelRoute{
		{PublicModel: "glm-5.3", MatchType: service.CompositeRouteMatchExact, Enabled: true, Endpoint: service.CompositeRouteEndpointChatCompletions, TargetPlatform: service.PlatformOpenAI, UpstreamModel: "large"},
		{PublicModel: "chat-only", MatchType: service.CompositeRouteMatchExact, Enabled: true, Endpoint: service.CompositeRouteEndpointChatCompletions, TargetPlatform: service.PlatformOpenAI, UpstreamModel: "large"},
		{PublicModel: "shared", MatchType: service.CompositeRouteMatchExact, Enabled: true, Endpoint: service.CompositeRouteEndpointResponses, TargetPlatform: service.PlatformOpenAI, UpstreamModel: "large"},
		{PublicModel: "shared", MatchType: service.CompositeRouteMatchExact, Enabled: true, Endpoint: service.CompositeRouteEndpointMessages, TargetPlatform: service.PlatformOpenAI, UpstreamModel: "small"},
	}}
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{group.ID: {account}}}, routes)
	for _, tc := range []struct {
		id             string
		window, output float64
	}{
		{"chat-only", 1000000, 64000}, {"shared", 300000, 32000}, {"glm-5.3", 1000000, 64000},
	} {
		rec := requestModelForTest(h, group, tc.id, "")
		require.Equal(t, http.StatusOK, rec.Code)
		var model map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &model))
		require.Equal(t, tc.window, model["context_window"], tc.id)
		require.Equal(t, tc.output, model["max_output_tokens"], tc.id)
	}
}
