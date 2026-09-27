// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_catalog.go
// @for       The model configuration Qoder publishes to an authenticated account,
//
//	and the hour of reuse that keeps it off every request.
//
// @uses      bytes, context, encoding/json, fmt, io, net/http, strings, sync, time.
// @reason    Qoder's chat body carries the vendor's own `model_config` object, and
//
//	the endpoint silently answers with a different model when the object it
//	is handed is wrong (the reference states this outright). The list is
//	also the only place a model that joined the vendor's catalogue after
//	the registry was generated can be named, so it is read from the
//	service rather than copied into a table.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// qoderCatalogTTL is the reference's own cache lifetime for the list, and
// qoderCatalogGroup the array it reads first. A group is searched before the others
// because `chat` is where the models this gateway routes live.
const (
	qoderCatalogTTL      = time.Hour
	qoderCatalogBodyByte = 4 << 20
)

// qoderCatalogEntry is one cached lookup: the vendor's object as it arrived, kept
// raw because the chat body hands it back unchanged. Re-marshalling a subset of its
// fields would drift from what the vendor's own client sends, and the endpoint
// decides by those fields.
type qoderCatalogEntry struct {
	config    json.RawMessage
	fetchedAt time.Time
}

// qoderCatalog is the per-connector cache. It is safe for concurrent use, and it
// holds no per-request state: the key names both the account's gateway and the
// model, which is the widest scope that is still correct.
type qoderCatalog struct {
	mu      sync.Mutex
	entries map[string]qoderCatalogEntry
	now     func() time.Time
}

func newQoderCatalog() *qoderCatalog {
	return &qoderCatalog{entries: map[string]qoderCatalogEntry{}, now: time.Now}
}

// modelConfig returns the vendor's configuration for one model key, reading the
// catalogue when the cached answer is absent or older than the TTL. An error means
// no configuration could be obtained; a caller must not send a chat without one,
// because the vendor would answer with a model nobody asked for.
func (c *Qoder) modelConfig(cred Credential, modelKey string) (json.RawMessage, error) {
	base, err := c.inferenceBase(cred)
	if err != nil {
		return nil, err
	}
	key := base + "|" + modelKey
	if cached, ok := c.catalog.read(key, c.catalog.now()); ok {
		return cached, nil
	}

	raw, err := c.fetchCatalog(cred, base)
	if err != nil {
		return nil, err
	}
	config, found := findQoderModelConfig(raw, modelKey)
	if !found {
		return nil, fmt.Errorf("provider %s: the vendor does not list model %q", c.entry.ID, modelKey)
	}
	c.catalog.write(key, config, c.catalog.now())
	return config, nil
}

// fetchCatalog asks the vendor's own list endpoint, signed as the account. The call
// is the one a live proof already exercises (draft 036 §5), so the shape read here
// is the shape that answered there.
func (c *Qoder) fetchCatalog(cred Credential, base string) ([]byte, error) {
	requestURL := base + qoderSigPathPrefix + qoderModelListPath
	// The catalogue is read while the caller's request is still being shaped, before any
	// context reaches a connector, so the read carries its own bound: an unanswered
	// list must not become the reason a call hangs.
	ctx, cancel := context.WithTimeout(context.Background(), qoderCatalogTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("provider %s: the model list request could not be built", c.entry.ID)
	}
	request.Header.Set("Accept", "application/json")
	if err := c.ApplyAuth(request, cred); err != nil {
		return nil, err
	}

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("provider %s: the model list could not be read: %w", c.entry.ID, err)
	}
	defer func() {
		// reason: the body is read in full below, so a close error adds nothing.
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider %s: the model list was refused with http %d", c.entry.ID, response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, qoderCatalogBodyByte))
	if err != nil {
		return nil, fmt.Errorf("provider %s: the model list could not be read: %w", c.entry.ID, err)
	}
	return raw, nil
}

// findQoderModelConfig picks the vendor's object for one key. Every top-level array
// is searched because the answer groups models by what they serve (`chat`, `inline`,
// `quest`, and others measured live) and a model can move between groups without the
// gateway learning it. The object is returned as the bytes the vendor sent: the chat
// body hands it back unchanged, so nothing here decodes fields it would have to
// re-pick.
func findQoderModelConfig(raw []byte, modelKey string) (json.RawMessage, bool) {
	var groups map[string]json.RawMessage
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, false
	}
	for _, name := range qoderCatalogGroupOrder(groups) {
		var entries []json.RawMessage
		if json.Unmarshal(groups[name], &entries) != nil {
			continue
		}
		for _, entry := range entries {
			var probe struct {
				Key string `json:"key"`
			}
			if json.Unmarshal(entry, &probe) == nil && probe.Key != "" && probe.Key == modelKey {
				return entry, true
			}
		}
	}
	return nil, false
}

// qoderCatalogGroupOrder reads `chat` first, then the remaining groups in a stable
// order so one catalogue cannot answer two different ways on two runs.
func qoderCatalogGroupOrder(groups map[string]json.RawMessage) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
		if name == "chat" {
			continue
		}
		names = append(names, name)
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	if _, ok := groups["chat"]; ok {
		return append([]string{"chat"}, names...)
	}
	return names
}

// inferenceBase is the host this account's traffic is served from, without the chat
// path: the catalogue lives on the same gateway as the chat endpoint it describes.
func (c *Qoder) inferenceBase(cred Credential) (string, error) {
	chatURL := strings.TrimSpace(c.entry.Transport.BaseURL)
	if chatURL == "" {
		return "", fmt.Errorf("provider %s: the chat base url is not declared", c.entry.ID)
	}
	value, err := c.credential(cred)
	if err != nil {
		return "", err
	}
	moved := chatURL
	if isQoderJobCredential(value) {
		moved, err = qoderSwapHost(chatURL, qoderChatBaseIntlDevice, qoderChatBaseIntlJob)
		if err != nil {
			return "", err
		}
	}
	return originOf(moved), nil
}

// originOf reduces a URL to scheme and host, the part a path-less endpoint like
// the catalogue shares with the chat call it describes.
func originOf(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return strings.TrimSuffix(rawURL, parsed.Path)
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (c *qoderCatalog) read(key string, now time.Time) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || now.Sub(entry.fetchedAt) > qoderCatalogTTL || len(entry.config) == 0 {
		return nil, false
	}
	return entry.config, true
}

func (c *qoderCatalog) write(key string, config json.RawMessage, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = qoderCatalogEntry{config: config, fetchedAt: now}
}

// qoderCatalogTimeout bounds the catalogue read on its own: it happens inside
// request shaping, where the call's deadline is already running, and an unanswered
// list must not become the reason a request hangs.
const qoderCatalogTimeout = 15 * time.Second
