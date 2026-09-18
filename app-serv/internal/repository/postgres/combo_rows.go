// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/combo_rows.go
// @for       The read and write codec for a combos row, plus the driver-error
//
//	translation the combo repository answers with.
//
// @uses      github.com/jackc/pgx/v5, internal/domain, encoding/json, time.
// @reason    A combo's models column is jsonb, so every read and every write
//
//	passes through the same pair of functions; keeping the codec and
//	the row scanner beside each other is what stops an encode and a
//	decode from drifting, and it keeps the repository's statement
//	surface readable on one screen.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability experimental
// @since     2026-09-17
package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// comboModelDocument is the jsonb element shape of combos.models. A concrete
// struct, so the column has one declared decode contract (AGENTS.md §1.4).
type comboModelDocument struct {
	Ref      string `json:"ref"`
	Priority int    `json:"priority"`
}

// encodeComboModels renders the ordered list as the jsonb array the column
// stores.
func encodeComboModels(models []domain.ComboModel) ([]byte, error) {
	documents := make([]comboModelDocument, 0, len(models))
	for _, model := range models {
		documents = append(documents, comboModelDocument{Ref: model.Ref(), Priority: model.Priority()})
	}
	encoded, err := json.Marshal(documents)
	if err != nil {
		return nil, fmt.Errorf("encoding combo models: %w", err)
	}
	return encoded, nil
}

// decodeComboModels reads the jsonb array back.
func decodeComboModels(raw []byte) ([]domain.ComboModel, error) {
	var documents []comboModelDocument
	if err := json.Unmarshal(raw, &documents); err != nil {
		return nil, fmt.Errorf("decoding combo models: %w", err)
	}
	models := make([]domain.ComboModel, 0, len(documents))
	for _, document := range documents {
		models = append(models, domain.RehydrateComboModel(document.Ref, document.Priority))
	}
	return models, nil
}

// scanCombo reads one row into a rehydrated aggregate.
func scanCombo(s pgx.Row) (domain.Combo, error) {
	var (
		id, name, strategy string
		stickyLimit        int
		judgeModel         *string
		rawModels          []byte
		createdAt          time.Time
		updatedAt          time.Time
	)
	if err := s.Scan(&id, &name, &strategy, &stickyLimit, &judgeModel, &rawModels, &createdAt, &updatedAt); err != nil {
		return domain.Combo{}, err
	}
	return buildCombo(id, name, strategy, stickyLimit, judgeModel, rawModels, createdAt, updatedAt)
}

// scanComboWithTotal reads a List row: the combo plus the window count.
func scanComboWithTotal(s pgx.Rows) (domain.Combo, int64, error) {
	var (
		id, name, strategy string
		stickyLimit        int
		judgeModel         *string
		rawModels          []byte
		createdAt          time.Time
		updatedAt          time.Time
		total              int64
	)
	if err := s.Scan(&id, &name, &strategy, &stickyLimit, &judgeModel, &rawModels, &createdAt, &updatedAt, &total); err != nil {
		return domain.Combo{}, 0, err
	}
	combo, err := buildCombo(id, name, strategy, stickyLimit, judgeModel, rawModels, createdAt, updatedAt)
	if err != nil {
		return domain.Combo{}, 0, err
	}
	return combo, total, nil
}

// buildCombo assembles the aggregate from its columns.
func buildCombo(id, name, strategy string, stickyLimit int, judgeModel *string, rawModels []byte, createdAt, updatedAt time.Time) (domain.Combo, error) {
	models, err := decodeComboModels(rawModels)
	if err != nil {
		return domain.Combo{}, err
	}
	judge := ""
	if judgeModel != nil {
		judge = *judgeModel
	}
	return domain.RehydrateCombo(id, name, domain.ComboStrategy(strategy), stickyLimit, judge,
		models, createdAt, updatedAt), nil
}

// nullString maps an absent optional column onto a SQL NULL, so "no judge"
// round-trips as NULL rather than as an empty string.
func nullString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// translateComboError maps a driver error to a domain error a caller can act
// on. Unrecognised errors are wrapped with table context; the wrapped text
// reaches logs only, because the handler renders domain.Message, never the
// chain (AGENTS.md §1.3).
func translateComboError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrComboNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return fmt.Errorf("%w: %s", domain.ErrComboExists, pgErr.ConstraintName)
	}
	return fmt.Errorf("combos: %w", err)
}
