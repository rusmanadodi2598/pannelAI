// Command app-serv adapts the registry's capability knowledge to the service
//
//	layer's predicate shape.
//
// @file      cmd/app-serv/vision_capability.go
// @for       The domain.VisionCapabilityCheck the vision adapter service is wired with.
// @uses      internal/domain, internal/registry.
// @reason    SPEC-API-001 §7.8 refuses a vision adapter model that cannot read images, and the predicate is a domain function type while the knowledge lives in internal/registry. Neither package may import the other: the registry is the static catalog and the domain is the business vocabulary. The composition root is the one layer allowed to know both, so the adapter is a few lines here rather than a dependency in either.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability stable
// @since     2026-09-18
package main

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// visionCapabilityCheck answers whether a catalog model reads images.
//
// It judges the model id the operator selected, the id the catalog lists, not
// the upstream id an override may point at: the panel asks whether this model,
// as this gateway names it, reads an image. The provider travels with it because
// registry.VisionCapable takes only a model id, which skips the per-provider
// override layer in the resolver where an entry's own answer differs from the
// family pattern.
func visionCapabilityCheck(ref domain.ModelRef) bool {
	return registry.Capabilities(ref.ProviderID(), ref.ModelID()).Vision
}
