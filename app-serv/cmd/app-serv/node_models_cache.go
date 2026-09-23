// Command app-serv remembers a provider node's model-list answer.
//
// @file      cmd/app-serv/node_models_cache.go
// @for       The short window a node's model list is reused for, so one panel
//
//	request costs one upstream read per node rather than one per row.
//
// @uses      internal/service, time.
// @reason    The provider index overlay rebuilds on every lookup
//
//	(cmd/app-serv/provider_index.go) and a catalog request walks the whole
//	index, so an uncached read would dial every custom node once per row.
//	Keeping the window apart from the request keeps each file about one
//	question — what to ask, and how long to believe the answer.
//
//	The window is short on purpose: an operator who adds a model upstream
//	and returns to the panel should see it without a restart.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/service"
)

// nodeModelCacheTTL is how long a successful read is reused.
const nodeModelCacheTTL = 5 * time.Minute

// nodeModelFailureTTL is how long a failed read is remembered. It is much
// shorter than a success, because the whole point of the short window is to stop
// a panel refresh from hammering an upstream that is down while still noticing
// when it comes back.
const nodeModelFailureTTL = 30 * time.Second

// cachedNodeModels is one node's remembered answer.
type cachedNodeModels struct {
	list      service.NodeModelList
	fetchedAt time.Time
	ttl       time.Duration
}

// cached returns a remembered answer that has not expired.
func (s *nodeModelSource) cached(nodeID string) (service.NodeModelList, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[nodeID]
	if !ok || s.clock().Sub(entry.fetchedAt) >= entry.ttl {
		return service.NodeModelList{}, false
	}
	return entry.list, true
}

// remember stores an answer and returns it, so every exit from a fetch goes
// through one place and no path forgets the cache.
func (s *nodeModelSource) remember(nodeID string, list service.NodeModelList, ttl time.Duration) service.NodeModelList {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[nodeID] = cachedNodeModels{list: list, fetchedAt: s.clock(), ttl: ttl}
	return list
}

// rememberFailure stores a fallback with the short failure window.
func (s *nodeModelSource) rememberFailure(nodeID, warning string) service.NodeModelList {
	return s.remember(nodeID, service.NodeModelList{
		Source:  service.ModelSourceRegistry,
		Warning: warning,
	}, nodeModelFailureTTL)
}
