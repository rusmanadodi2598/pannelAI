// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/endpoint_key_rows.go
// @for       The upstream_keys projection, its column lists, and the row-to-entity mapping in both directions.
// @uses      time, internal/domain.
// @reason    A credential is read and written through the endpoint aggregate, but the column spellings are shared by every key statement in the package, and COPY's argument order has to match its column list exactly. Keeping the projection, the copy list, and the two mappers in one file is what makes that coupling visible at one place instead of across three statements that can drift apart. Split from endpoint_keys.go to keep both inside the AGENTS.md §1.1 line budget.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-16
package postgres

import (
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// upstreamKeyColumns is the projection every key read uses, in scan order.
//
// The rotation columns are appended rather than woven in: they are read on the
// same load the selection already performs, and a second query for four scalar
// columns would be the N+1 shape AGENTS.md §1.7 forbids.
const upstreamKeyColumns = `id, endpoint_id, label, value_encrypted, key_hint,
	priority, status, last_used_at, last_error, consecutive_errors,
	rate_limited_until, created_at, updated_at,
	last_selected_at, consecutive_auth, consecutive_ratelimit, consecutive_billing,
	last_error_class, reset_source, signature_id,
	tags, disabled_until`

// upstreamKeyCopyColumns is the insert column list, which must match
// keyCopyRow's argument order exactly.
var upstreamKeyCopyColumns = []string{
	"id", "endpoint_id", "label", "value_encrypted", "key_hint", "priority",
	"status", "last_used_at", "last_error", "consecutive_errors",
	"rate_limited_until", "created_at", "updated_at",
	"last_selected_at", "consecutive_auth", "consecutive_ratelimit",
	"consecutive_billing", "last_error_class", "tags", "disabled_until",
}

// scanUpstreamKey reads one key row into a rehydrated entity.
func scanUpstreamKey(s scanner, extra ...any) (domain.UpstreamKey, error) {
	var (
		id, endpointID, label, valueEncrypted, keyHint string
		priority, consecutiveErrors                    int
		authRuns, rateRuns, billRuns                   int
		status                                         string
		lastUsedAt, lastSelectedAt, rateLimitedUntil   *time.Time
		lastError, lastErrorClass                      *string
		resetSource, signatureID                       *string
		tagsJSON                                       *string
		disabledUntil                                  *time.Time
		createdAt, updatedAt                           time.Time
	)
	dest := []any{
		&id, &endpointID, &label, &valueEncrypted, &keyHint, &priority, &status,
		&lastUsedAt, &lastError, &consecutiveErrors, &rateLimitedUntil,
		&createdAt, &updatedAt,
		&lastSelectedAt, &authRuns, &rateRuns, &billRuns, &lastErrorClass,
		&resetSource, &signatureID, &tagsJSON, &disabledUntil,
	}
	dest = append(dest, extra...)
	if err := s.Scan(dest...); err != nil {
		return domain.UpstreamKey{}, err
	}
	tags, err := unmarshalTags(tagsJSON)
	if err != nil {
		return domain.UpstreamKey{}, err
	}
	message := ""
	if lastError != nil {
		message = *lastError
	}
	class := ""
	if lastErrorClass != nil {
		class = *lastErrorClass
	}
	return domain.RehydrateUpstreamKey(id, endpointID, label, valueEncrypted,
		keyHint, priority, domain.UpstreamKeyStatus(status), lastUsedAt, message,
		consecutiveErrors,
		domain.KeyRotationState{
			LastSelectedAt:       lastSelectedAt,
			ConsecutiveAuth:      authRuns,
			ConsecutiveRateLimit: rateRuns,
			ConsecutiveBilling:   billRuns,
			LastErrorClass:       class,
			ResetSource:          derefText(resetSource),
			SignatureID:          derefText(signatureID),
		},
		rateLimitedUntil, createdAt, updatedAt,
		domain.OperatorState{Tags: tags, DisabledUntil: disabledUntil}), nil
}

// keyCopyRow renders one key as a COPY row. A null column is encoded as nil
// rather than an empty string, so "no last error" and "an empty last error"
// stay distinguishable in storage as they are in the entity.
func keyCopyRow(key domain.UpstreamKey) ([]any, error) {
	var lastError, lastErrorClass *string
	if key.LastError() != "" {
		message := key.LastError()
		lastError = &message
	}
	if key.LastErrorClass() != "" {
		class := key.LastErrorClass()
		lastErrorClass = &class
	}
	tagsJSON, err := marshalTags(key.Tags())
	if err != nil {
		return nil, err
	}
	return []any{
		key.ID(), key.EndpointID(), key.Label(), key.EncryptedValue(), key.Hint(),
		key.Priority(), string(key.Status()), key.LastUsedAt(), lastError,
		key.ConsecutiveErrors(), key.RateLimitedUntil(), key.CreatedAt(), key.UpdatedAt(),
		key.LastSelectedAt(), key.ConsecutiveAuth(), key.ConsecutiveRateLimit(),
		key.ConsecutiveBilling(), lastErrorClass, tagsJSON, key.DisabledUntil(),
	}, nil
}
