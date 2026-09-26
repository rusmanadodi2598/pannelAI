// Package handler adapts HTTP requests to service calls.
//
// @file      internal/handler/openapi_dto_registry_test.go
// @for       The contract-schema to Go-struct registry the DTO parity test walks.
// @uses      reflect, internal/schema.
// @reason    The registry is data, not a rule: it names which contract schema
// renders which struct. It lives apart from the comparison so a schema addition
// is one line in one place, and so both files stay inside the AGENTS.md §1.1
// budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     handler
// @stability experimental
// @since     2026-09-21
package handler

import (
	"reflect"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// dtoRegistry maps a contract schema name to the struct that renders it. Only
// schemas whose wire shape is one struct are listed: a composed schema (an
// embedded block flattened onto the wire, or a union) has no single struct to
// compare against, which is why the chat request's union members are listed
// individually rather than as the request they belong to.
func dtoRegistry() map[string]reflect.Type {
	types := []struct {
		name  string
		value any
	}{
		{"LoginRequest", schema.LoginRequest{}},
		{"ChangePasswordRequest", schema.ChangePasswordRequest{}},
		{"AuthStatusResponse", schema.AuthStatusResponse{}},
		{"GatewayKeyCreateRequest", schema.CreateGatewayKeyRequest{}},
		{"GatewayKeyUpdateRequest", schema.UpdateGatewayKeyRequest{}},
		{"GatewayKeyResponse", schema.GatewayKeyResponse{}},
		{"GatewayKeyList", schema.GatewayKeyList{}},
		{"OAuthStartRequest", schema.OAuthStartRequest{}},
		{"OAuthStartResponse", schema.OAuthStartResponse{}},
		{"OAuthRefreshRequest", schema.OAuthRefreshRequest{}},
		{"OAuthRefreshResponse", schema.OAuthRefreshResponse{}},
		{"OAuthStatusResponse", schema.OAuthStatusResponse{}},
		{"ProviderNodeCreateRequest", schema.CreateProviderNodeRequest{}},
		{"ProviderNodeUpdateRequest", schema.UpdateProviderNodeRequest{}},
		{"ProviderNodeResponse", schema.ProviderNodeResponse{}},
		{"ProviderNodeList", schema.ProviderNodeList{}},
		{"ProviderModelResponse", schema.ProviderModelResponse{}},
		{"ProviderModelList", schema.ProviderModelList{}},
		{"ProviderStatusSummary", schema.ProviderStatusSummaryDTO{}},
		{"EndpointKeyInput", schema.EndpointKeyInput{}},
		{"CreateEndpointRequest", schema.CreateEndpointRequest{}},
		{"UpdateEndpointRequest", schema.UpdateEndpointRequest{}},
		{"TestEndpointRequest", schema.TestEndpointRequest{}},
		{"AddEndpointKeyRequest", schema.AddEndpointKeyRequest{}},
		{"EndpointKeyUpdateRequest", schema.UpdateEndpointKeyRequest{}},
		{"BulkEndpointInput", schema.BulkEndpointInput{}},
		{"BulkCreateEndpointsRequest", schema.BulkCreateEndpointsRequest{}},
		{"BulkAddKeysRequest", schema.BulkAddKeysRequest{}},
		{"BulkOAuthAccountInput", schema.BulkOAuthAccountInput{}},
		{"BulkOAuthImportRequest", schema.BulkOAuthImportRequest{}},
		{"EndpointAccount", schema.EndpointAccountBody{}},
		{"EndpointAccountResponse", schema.EndpointAccountResponse{}},
		{"EndpointKeyResponse", schema.EndpointKeyResponse{}},
		{"EndpointKeyList", schema.EndpointKeyList{}},
		{"EndpointResponse", schema.EndpointResponse{}},
		{"EndpointList", schema.EndpointList{}},
		{"EndpointTestStatus", schema.EndpointTestStatusResponse{}},
		{"BulkResultRow", schema.BulkResultRow{}},
		{"BulkOAuthRow", schema.BulkOAuthRow{}},
		{"BulkOAuthResponse", schema.BulkOAuthResponse{}},
		{"ComboModelEntry", schema.ComboModelEntry{}},
		{"ComboRequest", schema.ComboRequest{}},
		{"ComboResponse", schema.ComboResponse{}},
		{"ComboList", schema.ComboList{}},
		{"ComboTestResult", schema.ComboTestResult{}},
		{"ComboTestResponse", schema.ComboTestResponse{}},
		{"VisionAdapterRequest", schema.ReplaceVisionAdapterRequest{}},
		{"VisionAdapterResponse", schema.VisionAdapterResponse{}},
		{"TokenSaverRequest", schema.ReplaceTokenSaverRequest{}},
		{"TokenSaverResponse", schema.TokenSaverResponse{}},
		{"TokenSaverRTK", schema.TokenSaverRTKRequest{}},
		{"TokenSaverHeadroom", schema.TokenSaverHeadroomRequest{}},
		{"TokenSaverLevel", schema.TokenSaverLevelRequest{}},
		{"ProxyRequest", schema.ProxyRequest{}},
		{"ProxyPatchRequest", schema.ProxyPatchRequest{}},
		{"ProxyCandidateRequest", schema.ProxyCandidateRequest{}},
		{"ProxyResponse", schema.ProxyResponse{}},
		{"ProxyList", schema.ProxyList{}},
		{"ProxyTestResponse", schema.ProxyTestResponse{}},
		{"ProxyTestStatusResponse", schema.ProxyTestStatusResponse{}},
		{"MediaOverrideRequest", schema.MediaOverrideRequest{}},
		{"MediaModelResponse", schema.MediaModelResponse{}},
		{"MediaProviderList", schema.MediaProviderList{}},
		{"ModelResponse", schema.ModelResponse{}},
		{"ModelCatalogResponse", schema.ModelCatalogResponse{}},
		{"CreateCustomModelRequest", schema.CreateCustomModelRequest{}},
		{"CustomModelResponse", schema.CustomModelResponse{}},
		{"CustomModelList", schema.CustomModelList{}},
		{"AliasResponse", schema.AliasResponse{}},
		{"AliasList", schema.AliasList{}},
		{"ReplaceAliasesRequest", schema.ReplaceAliasesRequest{}},
		{"AliasEntryRequest", schema.AliasEntry{}},
		{"DisabledModelResponse", schema.DisabledModelResponse{}},
		{"DisabledList", schema.DisabledList{}},
		{"ReplaceDisabledRequest", schema.ReplaceDisabledRequest{}},
		{"DisabledEntryRequest", schema.DisabledEntry{}},
		{"UsageTotals", schema.UsageTotalsResponse{}},
		{"UsageGroup", schema.UsageGroupResponse{}},
		{"UsageSummaryResponse", schema.UsageSummaryResponse{}},
		{"UsageBucket", schema.UsageBucketResponse{}},
		{"UsageTimeseriesResponse", schema.UsageTimeseriesResponse{}},
		{"UsageRecordResponse", schema.UsageRecordResponse{}},
		{"UsageRecordList", schema.UsageRecordList{}},
		{"UsageLiveActive", schema.UsageLiveActive{}},
		{"UsageLiveRecent", schema.UsageLiveRecent{}},
		{"UsageLiveFrame", schema.UsageLiveFrame{}},
		{"QuotaWindowResponse", schema.QuotaWindowResponse{}},
		{"QuotaWindowList", schema.QuotaWindowList{}},
		{"QuotaCapRequest", schema.QuotaCapRequest{}},
		{"QuotaCapResponse", schema.QuotaCapResponse{}},
		{"LogRecordResponse", schema.LogRecordResponse{}},
		{"LogList", schema.LogList{}},
		{"LogDetailResponse", schema.LogDetailResponse{}},
		{"LogPurgeResponse", schema.LogPurgeResponse{}},
		{"ConsoleResponse", schema.ConsoleResponse{}},
		{"SecuritySettingsResponse", schema.SecuritySettingsResponse{}},
		{"RoutingSettingsResponse", schema.RoutingSettingsResponse{}},
		{"NetworkSettingsResponse", schema.NetworkSettingsResponse{}},
		{"LoggingSettingsResponse", schema.LoggingSettingsResponse{}},
		{"SettingsResponse", schema.SettingsResponse{}},
		{"SystemInfo", schema.SystemInfo{}},
		{"HealthResponse", schema.HealthResponse{}},
		{"Page", schema.Page{}},
		{"ChatRequest", schema.ChatRequest{}},
		{"ChatRequestMessage", schema.ChatMessage{}},
		{"ContentPart", schema.ContentPart{}},
		{"ImageURL", schema.ImageURL{}},
		{"StreamOptions", schema.StreamOptions{}},
		{"JSONSchemaField", schema.JSONSchemaField{}},
		{"ResponseFormat", schema.ResponseFormat{}},
		{"Tool", schema.Tool{}},
		{"ToolFunction", schema.ToolFunction{}},
		{"ToolCall", schema.ToolCall{}},
		{"FunctionCall", schema.FunctionCall{}},
		{"ChatCompletionResponse", schema.ChatCompletionResponse{}},
		{"ChatChoice", schema.ChatChoice{}},
		{"ChatMessage", schema.ChatMessage{}},
		{"ChatUsage", schema.Usage{}},
		{"MessagesResponse", schema.MessagesResponse{}},
		{"MessagesUsage", schema.MessagesUsage{}},
		{"CountTokensRequest", schema.CountTokensRequest{}},
		{"CountTokensResponse", schema.CountTokensResponse{}},
		{"EmbeddingsRequest", schema.EmbeddingsRequest{}},
		{"EmbeddingsResponse", schema.EmbeddingsResponse{}},
		{"EmbeddingObject", schema.EmbeddingObject{}},
		{"ModelObject", schema.ModelObject{}},
		{"ModelListResponse", schema.ModelList{}},
		{"VoiceObject", schema.VoiceObject{}},
		{"VoiceList", schema.VoiceList{}},
		{"SpeechRequest", schema.SpeechRequest{}},
		{"SpeechResponse", schema.SpeechResponse{}},
		{"ImageRequest", schema.ImageBody{}},
		{"VideoRequest", schema.VideoBody{}},
		{"MediaGenerationObject", schema.MediaGenerationObject{}},
		{"MediaGenerationResponse", schema.MediaGenerationResponse{}},
		{"SearchRequest", schema.SearchRequest{}},
		{"SearchResult", schema.SearchResult{}},
		{"SearchUsage", schema.SearchUsage{}},
		{"SearchMetrics", schema.SearchMetrics{}},
		{"SearchResponse", schema.SearchResponse{}},
	}
	registry := make(map[string]reflect.Type, len(types))
	for _, entry := range types {
		registry[entry.name] = reflect.TypeOf(entry.value)
	}
	return registry
}
