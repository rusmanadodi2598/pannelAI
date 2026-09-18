// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/connectors.go
// @for       The connector registry: how a provider id resolves to the plugin
//
//	that handles it, and the fallback when none is registered.
//
// @uses      internal/registry, fmt, sort, sync, strings.
// @reason    The user's requirement is that a provider can be patched or added
//
//	without disturbing the others. That only holds if the core looks
//	connectors up by id at runtime instead of branching on the id in
//	shared code, so this file owns the lookup and nothing else knows
//	which providers have a specialized connector.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package provider

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Connectors holds the registered plugins, keyed by provider id. It is built
// once at boot and read concurrently thereafter, so registration happens before
// the server starts and the map is never written again.
type Connectors struct {
	byID map[string]Plugin

	// fallback is used for a provider with no registered connector. A provider
	// that speaks a standard wire format needs no connector at all, which is
	// what keeps the registry from having to grow one file per vendor.
	fallback func(provider registry.Provider) Plugin
}

// NewConnectors builds the registry from the plugins a service wires in and the
// factory that handles everything else.
//
// A duplicate provider id is refused rather than silently overwritten: two
// connectors claiming one provider means the behaviour depends on wiring order,
// which is exactly the ambiguity this registry exists to remove.
func NewConnectors(fallback func(registry.Provider) Plugin, plugins ...Plugin) (*Connectors, error) {
	if fallback == nil {
		return nil, fmt.Errorf("provider: a fallback connector factory is required")
	}
	byID := make(map[string]Plugin, len(plugins))
	for _, plugin := range plugins {
		if plugin == nil {
			return nil, fmt.Errorf("provider: a registered connector is nil")
		}
		id := strings.TrimSpace(plugin.ProviderID())
		if id == "" {
			return nil, fmt.Errorf("provider: a registered connector declares no provider id")
		}
		if _, dup := byID[id]; dup {
			return nil, fmt.Errorf("provider: provider id %q is registered twice", id)
		}
		byID[id] = plugin
	}
	return &Connectors{byID: byID, fallback: fallback}, nil
}

// For returns the connector that handles a provider. A provider with no
// registered connector gets the fallback, so adding a provider to the embedded
// registry is enough to make it routable when it speaks a standard format.
func (c *Connectors) For(provider registry.Provider) Plugin {
	if plugin, ok := c.byID[provider.ID]; ok {
		return plugin
	}
	return c.fallback(provider)
}

// Registered reports whether a provider has a specialized connector. It is what
// the panel shows as a provider's support level, and what a maintainer greps
// when wondering whether a provider has custom handling.
func (c *Connectors) Registered(providerID string) bool {
	_, ok := c.byID[providerID]
	return ok
}

// Count reports how many specialized connectors are registered.
func (c *Connectors) Count() int { return len(c.byID) }

// IDs lists the provider ids with a specialized connector, sorted so a report
// or a test comparing the set is stable.
func (c *Connectors) IDs() []string {
	ids := make([]string, 0, len(c.byID))
	for id := range c.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Unsupported lists providers in the given registry index that would use the
// fallback. It exists so the gap is visible: a provider that needs custom
// handling but has none is a routing failure waiting for traffic, not a
// discovery to make at runtime.
func (c *Connectors) Unsupported(idx *registry.Index) []string {
	var unsupported []string
	for _, entry := range idx.All() {
		if _, ok := c.byID[entry.ID]; ok {
			continue
		}
		// A connector-free provider is only acceptable when the fallback can
		// actually serve it: a plain API-key provider on a supported wire
		// format. Anything else needs its own connector.
		if defaultServed(entry) {
			continue
		}
		unsupported = append(unsupported, entry.ID)
	}
	sort.Strings(unsupported)
	return unsupported
}

// defaultServed reports whether the fallback connector can service a provider
// without custom handling.
func defaultServed(entry registry.Provider) bool {
	// A provider whose credentials sit behind a bespoke exchange needs a
	// connector regardless of format, because the fallback only knows a static
	// key or a bearer token.
	if entry.OAuth != nil && entry.OAuth.RequiresCustomExchange() {
		return false
	}
	// Otherwise the registry's format rule decides, so the panel's "can this
	// provider answer?" and this lookup's "does it need a connector?" cannot
	// disagree.
	return entry.IsChatRoutable()
}

// pluginOnce guards lazy construction of the process-wide registry, so a caller
// that needs the connectors before boot has finished still gets one instance.
var (
	pluginOnce sync.Once
	pluginSet  *Connectors
)

// Register installs the process-wide connector registry. It is called once from
// the composition root and refuses a second call, because replacing the set at
// runtime would make behaviour depend on call order.
func Register(connectors *Connectors) error {
	if connectors == nil {
		return fmt.Errorf("provider: connectors must not be nil")
	}
	assigned := false
	pluginOnce.Do(func() {
		pluginSet = connectors
		assigned = true
	})
	if !assigned {
		return fmt.Errorf("provider: the connector registry is already installed")
	}
	return nil
}

// Lookup returns the process-wide connector registry, or an error when the
// composition root has not installed one. Failing loudly is deliberate: a nil
// registry would route every provider through a zero-value plugin.
func Lookup() (*Connectors, error) {
	if pluginSet == nil {
		return nil, fmt.Errorf("provider: the connector registry has not been installed")
	}
	return pluginSet, nil
}
