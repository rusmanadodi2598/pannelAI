// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/load.go
// @for       Strict decoding of the embedded registry and its identifier index.
// @uses      embed, gopkg.in/yaml.v3, fmt, net/url, sort, strings.
// @reason    SPEC-API-001 §6 makes the registry embedded static config and §7.4
//
//	serves it over HTTP, so it is decoded exactly once and never
//	mutated afterwards. Decoding is strict because a key that does not
//	match a struct tag is a field the router silently stops honouring;
//	turning that into a boot failure is the only way drift is noticed.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-17
package registry

import (
	"bytes"
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// embedded carries the registry inside the binary, so a deployed app-serv needs
// no registry file on disk (SPEC-API-001 §6).
//
//go:embed registry.yaml
var embedded embed.FS

// registryFile is the embedded document's name.
const registryFile = "registry.yaml"

// Auth mode values for Provider.AuthType and Provider.AuthModes.
const (
	AuthAPIKey = "api_key"
	AuthOAuth  = "oauth"
	AuthNone   = "no_auth"
)

// DefaultFormat is the wire format a provider uses when it declares none; the
// reference's registry schema applies the same default (SPEC-API-001 §5).
const DefaultFormat = "openai"

// CustomPriority places a user-defined node after every built-in provider, so
// adding one never silently displaces a curated entry.
const CustomPriority = 100

// Index is the immutable, read-only view of the registry. It is built once at
// boot and shared by every request; nothing here mutates after NewIndex.
type Index struct {
	revision  string
	providers []Provider
	byID      map[string]int
	byName    map[string]string
}

// Load decodes the embedded registry into an Index, or fails with the reason a
// document could not be used.
func Load() (*Index, error) {
	raw, err := embedded.ReadFile(registryFile)
	if err != nil {
		return nil, fmt.Errorf("registry: reading embedded %s: %w", registryFile, err)
	}
	doc, err := decode(raw)
	if err != nil {
		return nil, err
	}
	return NewIndex(doc)
}

// decode reads a document strictly: an unknown key is an error rather than a
// silently ignored field, because the field it names is one the router would
// otherwise stop honouring without a word.
func decode(raw []byte) (Document, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Document{}, fmt.Errorf("registry: document is empty")
	}

	var doc Document
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("registry: decoding document: %w", err)
	}
	if len(doc.Providers) == 0 {
		return Document{}, fmt.Errorf("registry: document declares no providers")
	}
	return doc, nil
}

// NewIndex validates the document and builds the lookup tables.
//
// Name resolution has one precedence rule, and it follows the reference rather
// than intuition: an alias wins over another provider's bare id.
//
// The reference builds one flat alias table from every provider's
// `uiAlias || alias` and resolves with `ALIAS_TO_ID[token] || token`, so a
// provider whose id is also someone else's alias is UNREACHABLE by that id.
// That case exists in the data: `mimo-free` declares `alias: mmf` while a
// separate hidden entry has `id: mmf`, and both point at the same base URL and
// model. Resolving id-first would summon the hidden entry and silently change
// which provider answers "mmf/..."; alias-first reaches the same upstream the
// reference does.
//
// Two *aliases* colliding stays fatal: neither is canonical, the reference
// would resolve by insertion order, and there is no basis to prefer one.
func NewIndex(doc Document) (*Index, error) {
	idx := &Index{
		revision:  doc.Revision,
		providers: make([]Provider, 0, len(doc.Providers)),
		byID:      make(map[string]int, len(doc.Providers)),
		byName:    make(map[string]string, len(doc.Providers)),
	}

	for _, provider := range doc.Providers {
		if err := provider.validate(); err != nil {
			return nil, err
		}
		if _, dup := idx.byID[provider.ID]; dup {
			return nil, fmt.Errorf("registry: provider id %q is declared twice", provider.ID)
		}
		idx.byID[provider.ID] = len(idx.providers)
		idx.providers = append(idx.providers, provider)
		idx.byName[provider.ID] = provider.ID
	}

	// aliasOwner tracks which provider claimed a name as an *alias*, so a
	// collision between two aliases is still caught while an alias shadowing an
	// id is allowed.
	aliasOwner := make(map[string]string, len(idx.providers))
	for _, provider := range idx.providers {
		provider.Transport.Format = normalizeFormat(provider.Transport.Format)
		provider.BuildAuthType()
		idx.providers[idx.byID[provider.ID]] = provider

		for _, alias := range provider.aliasNames() {
			if owner, taken := aliasOwner[alias]; taken && owner != provider.ID {
				return nil, fmt.Errorf("registry: alias %q maps to both %q and %q", alias, owner, provider.ID)
			}
			aliasOwner[alias] = provider.ID
			idx.byName[alias] = provider.ID
		}
	}
	return idx, nil
}

// validate rejects an entry the rest of the system cannot route.
func (p Provider) validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("registry: a provider entry has no id")
	}
	if strings.TrimSpace(p.Category) == "" {
		return fmt.Errorf("registry: provider %q has no category", p.ID)
	}
	return nil
}

// BuildAuthType derives the provider's default authentication mode when the
// document does not state one.
//
// The reference leaves authType unset for most entries and lets the UI infer it
// from the provider's category, which puts the same rule in more than one
// place. Deriving it once at load keeps one answer: no_auth for a provider that
// needs no credential, oauth when it has a flow, otherwise api_key.
//
// no_auth is read from either level, because the reference writes it on the
// provider for some entries and on the transport for others; reading only one
// level would report a credential-free provider as needing a key, and routing
// would then refuse a request it could have served.
func (p *Provider) BuildAuthType() {
	p.NoAuth = p.NoAuth || p.Transport.NoAuth
	if p.Transport.AuthType != "" && p.AuthType == "" {
		p.AuthType = p.Transport.AuthType
	}
	if p.AuthType != "" {
		return
	}
	switch {
	case p.NoAuth:
		p.AuthType = AuthNone
	case p.OAuth != nil:
		p.AuthType = AuthOAuth
	default:
		p.AuthType = AuthAPIKey
	}
}

// aliasNames returns only the non-canonical identifiers: the short alias and
// any extra aliases. The id is deliberately excluded, so the precedence rule in
// NewIndex has exactly one place to live.
func (p Provider) aliasNames() []string {
	aliases := make([]string, 0, 1+len(p.Aliases))
	if p.Alias != "" && p.Alias != p.ID {
		aliases = append(aliases, p.Alias)
	}
	for _, alias := range p.Aliases {
		if alias == "" || alias == p.ID || alias == p.Alias {
			continue
		}
		aliases = append(aliases, alias)
	}
	return aliases
}

// normalizeFormat applies the declared default so no reader has to know it.
func normalizeFormat(format string) string {
	if strings.TrimSpace(format) == "" {
		return DefaultFormat
	}
	return format
}
