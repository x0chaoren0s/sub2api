package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountHandlerSyncUpstreamModelsUpdatesSchedulingUnlessCapabilitiesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name       string
		query      string
		wantStatus int
		wantModel  bool
	}{
		{"manual sync", "", http.StatusOK, true},
		{"capabilities only", "?capabilities_only=true", http.StatusOK, false},
		{"invalid option", "?capabilities_only=invalid", http.StatusBadRequest, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &service.Account{ID: 7, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Credentials: map[string]any{
					"base_url": "https://relay.example.com/v1", "api_key": "sync-secret",
					"model_mapping": map[string]any{"GLM-5.3": "GLM-5.3", "retired": "retired"},
				}}
			repo := &grokDiagCatalogRepo{account: cloneGrokDiagAccount(account)}
			upstream := &grokDiagHandlerUpstream{responses: []*http.Response{{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{"data":[
					{"id":"GLM-5.3","reasoning":false,"input_modalities":["text"],"context_window":1048576},
					{"id":"GLM-5.4","reasoning":false,"input_modalities":["text"],"context_window":1048576}
				]}`)),
			}}}
			adminSvc := &simpleModeAccountService{stubAdminService: newStubAdminService(), account: *account}
			router := newGrokDiagSyncRouter(adminSvc, repo, upstream)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/7/models/sync-upstream"+tt.query, nil))
			require.Equal(t, tt.wantStatus, rec.Code, rec.Body.String())
			stored, err := repo.GetByID(context.Background(), 7)
			require.NoError(t, err)
			require.Equal(t, tt.wantModel, stored.IsModelSupported("GLM-5.4"))
			require.Equal(t, !tt.wantModel, stored.IsModelSupported("retired"))
			require.True(t, stored.IsModelSupported("GLM-5.3"))
			require.Equal(t, "sync-secret", stored.Credentials["api_key"])
			require.NotContains(t, rec.Body.String(), "sync-secret")
		})
	}
}
