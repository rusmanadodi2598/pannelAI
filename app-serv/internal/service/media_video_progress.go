// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_video_progress.go
// @for       Reading one job's progress member into the envelope's percentage, whichever shape the provider wrote.
// @uses      encoding/json, strconv, strings.
// @reason    TokenRouter answers progress as a percentage string ("100%") while the envelope reports an integer, so the conversion sits apart from the reader that needs it and media_video_read.go stays inside the AGENTS.md §1.1 budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-21
package service

import (
	"encoding/json"
	"strconv"
	"strings"
)

// videoProgressFrom reads a job's progress member, which TokenRouter writes as
// a percentage string and other providers as a number. A value neither shape
// reads is dropped rather than failing a job that already answered: the
// percentage is cosmetic, and the envelope's required members are the id and
// the state.
func videoProgressFrom(raw json.RawMessage) *int {
	if len(raw) == 0 {
		return nil
	}
	var number *float64
	if err := json.Unmarshal(raw, &number); err == nil {
		if number == nil {
			return nil
		}
		value := int(*number)
		return &value
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil
	}
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "%"))
	value, err := strconv.Atoi(text)
	if err != nil {
		return nil
	}
	return &value
}
