package service

import (
	"context"
	"strings"
)

// ModelTokenLimitsForGroup advertises only token limits shared by the accounts
// serving each public model ID, using the same routing rules as the Codex catalog.
func (s *GatewayService) ModelTokenLimitsForGroup(ctx context.Context, group *Group, platform string, modelIDs []string) map[string]map[string]int64 {
	limits := make(map[string]map[string]int64)
	if s == nil || s.accountRepo == nil || group == nil {
		return limits
	}
	if strings.TrimSpace(platform) == "" {
		platform = group.Platform
	}
	_, accounts, err := loadCodexGroupCatalogAccounts(ctx, s.accountRepo, group.ID)
	if err != nil {
		return limits
	}
	var routes []CompositeModelRoute
	routesAvailable := true
	if platform == PlatformComposite && s.compositeResolver != nil && s.compositeResolver.repo != nil {
		routes, err = s.compositeResolver.repo.ListByGroup(ctx, group.ID, false)
		routesAvailable = err == nil
	}
	for _, id := range modelIDs {
		metadata, ok := groupCodexModelMetadata(platform, id, accounts, group, routes, routesAvailable)
		if platform == PlatformComposite {
			metadata, ok = compositeModelTokenMetadata(id, accounts, group, routes, routesAvailable)
		}
		if !ok {
			continue
		}
		fields := make(map[string]int64)
		if metadata.ContextWindow > 0 {
			fields["context_window"] = metadata.ContextWindow
		}
		if metadata.MaxContextWindow > 0 {
			fields["max_context_window"] = metadata.MaxContextWindow
		}
		if metadata.MaxInputTokens > 0 {
			fields["max_input_tokens"] = metadata.MaxInputTokens
		}
		if metadata.MaxOutputTokens > 0 {
			fields["max_output_tokens"] = metadata.MaxOutputTokens
			fields["max_tokens"] = metadata.MaxOutputTokens
		}
		limits[id] = fields
	}
	return limits
}

func compositeModelTokenMetadata(id string, accounts []Account, group *Group, routes []CompositeModelRoute, available bool) (codexModelMetadataOverride, bool) {
	if !available {
		return codexModelMetadataOverride{}, false
	}
	candidates := make([]UpstreamModelMetadata, 0)
	for _, endpoint := range []string{CompositeRouteEndpointResponses, CompositeRouteEndpointMessages, CompositeRouteEndpointChatCompletions, CompositeRouteEndpointGemini, CompositeRouteEndpointEmbeddings, CompositeRouteEndpointImages} {
		platform, upstream, resolved := resolveCodexCompositeModelTarget(id, accounts, nil, true)
		if route, matched := matchCompositeRoute(routes, id, endpoint); matched {
			platform, upstream, resolved = route.TargetPlatform, strings.TrimSpace(route.UpstreamModel), true
			if upstream == "" {
				upstream = id
			}
		}
		if !resolved {
			continue
		}
		metadata, ok := groupModelMetadataForTarget(platform, id, upstream, accounts, group)
		if !ok {
			continue
		}
		candidates = append(candidates, metadata.UpstreamModelMetadata)
	}
	if len(candidates) == 0 {
		return codexModelMetadataOverride{}, false
	}
	return intersectUpstreamModelMetadata(id, candidates), true
}
