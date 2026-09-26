// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/settings_apply.go
// @for       The write-only helpers a settings patch is applied through.
//
// @uses      (no imports).
// @reason    SPEC-API-001 §7.14 makes a nil field mean "leave unchanged", so
//
//	every group's application reduces to "write only when carried". The
//	helpers are split from settings_patch.go because the whole-map groups
//	keep arriving and that file is one reviewable unit only while it
//	states the rules rather than the plumbing.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability experimental
// @since     2026-09-26
package domain

// applyBool writes a value only when the patch carried one.
func applyBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}

// applyInt writes a value only when the patch carried one.
func applyInt(dst *int, src *int) {
	if src != nil {
		*dst = *src
	}
}

// applyString writes a value only when the patch carried one.
func applyString(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}

// applyProviderProxies writes the per-provider binding map. It is replaced
// wholesale when the patch carries one, so a deleted entry stays deleted; the
// copy keeps the stored document from sharing the caller's map.
func applyProviderProxies(dst *map[string]ProviderProxy, src *map[string]ProviderProxy) {
	if src == nil {
		return
	}
	replaced := make(map[string]ProviderProxy, len(*src))
	for id, entry := range *src {
		replaced[id] = entry
	}
	*dst = replaced
}

// applyToggle writes one saver toggle group only when the patch carried it.
func applyToggle(dst *TokenSaverToggle, src *TokenSaverTogglePatch) {
	if src == nil {
		return
	}
	applyBool(&dst.Enabled, src.Enabled)
	applyString(&dst.Level, src.Level)
}

// applyRTK writes the native engine's group. The filter list is replaced
// wholesale, so an empty list in the patch is a real value (clear the allowlist)
// rather than "leave unchanged", which the nil pointer already means.
func applyRTK(dst *TokenSaverRTK, src *TokenSaverRTKPatch) {
	if src == nil {
		return
	}
	applyBool(&dst.Enabled, src.Enabled)
	if src.Filters != nil {
		replaced := make([]string, len(*src.Filters))
		copy(replaced, *src.Filters)
		dst.Filters = replaced
	}
}
