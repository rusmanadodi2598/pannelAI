// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/qoder_catalog_lookup.go
// @for       The model lookup path over the cached catalogue, with one fetch per host.
// @uses      encoding/json, fmt, sync, time
// @reason    The model key is client-supplied, so a name the vendor does not list used to turn every
//
//	request into a fresh full-catalogue read: several concurrent requests read the same
//	4 MiB document at once, and a repeated unknown name never learns to stop asking.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// qoderCatalogMissTTL is how long a model the vendor does not list stays refused
// without re-reading the catalogue. It is short on purpose: a model the vendor
// adds becomes routable inside one window rather than one hour.
const qoderCatalogMissTTL = 60 * time.Second

// catalogFetch is one in-flight catalogue read that other callers join.
// raw and err are written before ready is closed, so a waiter that returns
// through the channel sees them without locking.
type catalogFetch struct {
	ready chan struct{}
	raw   []byte
	err   error
}

// beginFetch returns the fetch for this host and whether the caller must run it.
// A false second return means somebody is already reading.
func (c *qoderCatalog) beginFetch(base string) (*catalogFetch, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if running, ok := c.inflight[base]; ok {
		return running, false
	}
	fetch := &catalogFetch{ready: make(chan struct{})}
	c.inflight[base] = fetch
	return fetch, true
}

// completeFetch publishes one fetch's answer and drops it from the inflight set.
func (c *qoderCatalog) completeFetch(base string, fetch *catalogFetch, raw []byte, err error) {
	c.mu.Lock()
	delete(c.inflight, base)
	c.mu.Unlock()
	fetch.raw, fetch.err = raw, err
	close(fetch.ready)
}

// modelConfig returns the vendor's configuration for one model key, reading the
// catalogue when the cached answer is absent or older than the TTL. An error means
// no configuration could be obtained and a caller must not send a chat without
// one, because the vendor would answer with a model nobody asked for. ctx is the
// caller's own wait, not the fetch's: one fetch answers every concurrent lookup,
// so aborting it when its leader's client leaves would break the callers still
// waiting. A request whose client went away stops paying for a document it will no
// longer deliver.
func (c *Qoder) modelConfig(ctx context.Context, cred Credential, modelKey string) (json.RawMessage, error) {
	base, err := c.inferenceBase(cred)
	if err != nil {
		return nil, err
	}
	key := base + "|" + modelKey
	now := c.catalog.now()
	if cached, ok := c.catalog.read(key, now); ok {
		return cached, nil
	}
	if c.catalog.isMiss(key, now) {
		return nil, fmt.Errorf("provider %s: the vendor does not list model %q", c.entry.ID, modelKey)
	}

	fetch, leader := c.catalog.beginFetch(base)
	if leader {
		raw, fetchErr := c.fetchCatalog(ctx, cred, base)
		c.catalog.completeFetch(base, fetch, raw, fetchErr)
	} else {
		select {
		case <-fetch.ready:
		case <-ctx.Done():
			return nil, fmt.Errorf("provider %s: the caller left before the model catalogue answered: %w", c.entry.ID, ctx.Err())
		}
	}
	if fetch.err != nil {
		return nil, fetch.err
	}

	config, found := findQoderModelConfig(fetch.raw, modelKey)
	if !found {
		c.catalog.rememberMiss(key, now)
		return nil, fmt.Errorf("provider %s: the vendor does not list model %q", c.entry.ID, modelKey)
	}
	c.catalog.write(key, config, c.catalog.now())
	return config, nil
}

// isMiss reports a model key refused recently, so an unknown name costs one
// catalogue read per window instead of one per request.
func (c *qoderCatalog) isMiss(key string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	refused, ok := c.misses[key]
	if !ok {
		return false
	}
	if now.Sub(refused) > qoderCatalogMissTTL {
		delete(c.misses, key)
		return false
	}
	return true
}

// qoderMaxRememberedMisses bounds the miss set. The key is client-supplied, so an
// unbounded map would let a caller naming fresh models on every request grow the
// process for as long as they keep asking.
const qoderMaxRememberedMisses = 256

func (c *qoderCatalog) rememberMiss(key string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.misses) >= qoderMaxRememberedMisses {
		// Drop what has aged out; if the caller is still naming fresh keys, the
		// window is short enough that starting over costs one catalogue read.
		for recorded, when := range c.misses {
			if now.Sub(when) > qoderCatalogMissTTL {
				delete(c.misses, recorded)
			}
		}
		if len(c.misses) >= qoderMaxRememberedMisses {
			clear(c.misses)
		}
	}
	c.misses[key] = now
}
