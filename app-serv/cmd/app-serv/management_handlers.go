// Command app-serv assembles the management handler set.
//
// @file      cmd/app-serv/management_handlers.go
// @for       The one place the management handlers are built, so the wiring
//
//	function stays about composing services.
//
// @uses      internal/handler, internal/service.
// @reason    buildManagement was approaching the AGENTS.md §1.1 line limit, and
//
//	the handler construction is the part that grows every time a route is
//	added: it is a list, not a decision. Keeping it apart lets the wiring
//	file stay readable as the set of services it composes.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/handler"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// managementHandlerInputs is every service a handler is built from.
//
// It is a struct rather than a parameter list because the set is a dozen wide and
// two of the fields are themselves groupings: a positional call would be a row of
// same-typed arguments where one swap compiles and misroutes.
type managementHandlerInputs struct {
	Provider      *service.ProviderService
	Validation    *service.CredentialValidationService
	Endpoint      *service.EndpointService
	Node          *service.NodeService
	Catalog       *service.ModelCatalogService
	Combo         *service.ComboService
	ComboTest     *service.ComboTestService
	Proxy         *handler.ProxyHandler
	Media         *handler.MediaProviderHandler
	Plane         dataPlane
	Vision        *service.VisionAdapterService
	TokenSaver    *service.TokenSaverService
	Settings      *service.SettingsService
	OAuth         *handler.OAuthHandler
	Observability observability
	Flusher       *service.QuotaFlusher
	Retention     *service.LogRetentionWorker
	RefreshWorker *service.OAuthRefreshWorker
}

// buildManagementHandlers assembles the §7 route groups' handlers and the workers
// the boot sequence starts after the server is listening.
func buildManagementHandlers(in managementHandlerInputs) (managementDeps, error) {
	return managementDeps{
		Provider:      handler.NewProviderHandler(in.Provider),
		Validate:      handler.NewProviderValidateHandler(in.Validation),
		Endpoint:      handler.NewEndpointHandler(in.Endpoint),
		EndpointKey:   handler.NewEndpointKeyHandler(in.Endpoint),
		EndpointBulk:  handler.NewEndpointBulkHandler(in.Endpoint),
		OAuth:         in.OAuth,
		Node:          handler.NewProviderNodeHandler(in.Node),
		Model:         handler.NewModelHandler(in.Catalog),
		Combo:         handler.NewComboHandler(in.Combo),
		ComboTest:     handler.NewComboTestHandler(in.ComboTest),
		Proxy:         in.Proxy,
		MediaProvider: in.Media,
		Media:         handler.NewMediaHandler(in.Plane.Media, in.Plane.Chat),
		VisionAdapter: handler.NewVisionAdapterHandler(in.Vision),
		TokenSaver:    handler.NewTokenSaverHandler(in.TokenSaver),
		Usage:         handler.NewUsageHandler(in.Observability.Usage),
		UsageLive:     handler.NewUsageLiveHandler(in.Observability.UsageLive),
		Quota:         handler.NewQuotaHandler(in.Observability.Quota),
		Log:           handler.NewLogHandler(in.Observability.Log),
		Settings:      handler.NewSettingsHandler(in.Settings),
		Chat:          handler.NewChatHandler(in.Plane.Chat),
		Embeddings:    handler.NewEmbeddingsHandler(in.Plane.Embeddings, in.Plane.Chat),
		// The estimate route dials no upstream, so it is built over its own
		// stateless service and borrows the §4 key rule from the chat service.
		TokenCount:         handler.NewTokenCountHandler(service.NewTokenCountService(), in.Plane.Chat),
		QuotaFlusher:       in.Flusher,
		LogRetention:       in.Retention,
		OAuthRefresh:       in.RefreshWorker,
		UsageEvents:        in.Observability.Publisher,
		UsageEventConsumer: in.Observability.Consumer,
	}, nil
}
