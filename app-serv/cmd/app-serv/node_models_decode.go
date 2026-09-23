// Command app-serv parses a provider node's model-list answer.
//
// @file      cmd/app-serv/node_models_decode.go
// @for       The response shapes a compatible node's models endpoint answers
//
//	with, and the projection onto registry models.
//
// @uses      internal/registry, encoding/json, errors, io, strings.
// @reason    SPEC-API-001 §7.4 serves a node's models, and a compatible upstream
//
//	answers one of four shapes: a bare array, or an object under `data`,
//	`models`, or `results` — the set the reference accepts in
//	parseOpenAIStyleModels. Keeping the parsing apart from the request
//	keeps the adapter's own file about the dial and its policy, and keeps
//	both inside the AGENTS.md §1.1 line budget.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-23
package main

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// decodeNodeModels reads the model-list shapes the reference accepts
// (`parseOpenAIStyleModels`): a bare array, or an object under `data`, `models`,
// or `results`.
//
// An entry with no id is dropped rather than zero-valued: a nameless model is
// not a model string an operator can use, and a blank row in the panel is worse
// than a shorter list.
func decodeNodeModels(reader io.Reader) ([]registry.Model, error) {
	var payload struct {
		Data    []nodeModelEntry `json:"data"`
		Models  []nodeModelEntry `json:"models"`
		Results []nodeModelEntry `json:"results"`
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		// A bare array is the other accepted shape, so it is tried only after
		// the object shape fails rather than as a pre-check on the first byte.
		var bare []nodeModelEntry
		if arrayErr := json.Unmarshal(raw, &bare); arrayErr != nil {
			return nil, errors.Join(err, arrayErr)
		}
		return entriesToModels(bare), nil
	}
	for _, candidate := range [][]nodeModelEntry{payload.Data, payload.Models, payload.Results} {
		if len(candidate) > 0 {
			return entriesToModels(candidate), nil
		}
	}
	return nil, nil
}

// nodeModelEntry is one model in an upstream's answer.
type nodeModelEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// entriesToModels projects the entries onto registry models, dropping nameless
// rows and defaulting the display name to the id so the panel never renders a
// blank cell.
func entriesToModels(entries []nodeModelEntry) []registry.Model {
	out := make([]registry.Model, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			id = strings.TrimSpace(entry.Slug)
		}
		if id == "" {
			continue
		}
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = id
		}
		out = append(out, registry.Model{ID: id, Name: name})
	}
	return out
}
