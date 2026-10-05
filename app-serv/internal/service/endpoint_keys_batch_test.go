// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/endpoint_keys_batch_test.go
// @for       The all-or-nothing property of a bulk key add and its row attribution.
// @uses      strconv, testing, internal/domain.
// @reason    One rejected row must not leave the others attached, and the client
//
//	must be told which row failed; both are bulk-only rules with no
//	single-key counterpart.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

// TestEndpointService_AddKeyBatchIsAllOrNothing is the §8.1 rule: a batch with one bad
// row writes nothing at all, and a valid batch writes every row.
func TestEndpointService_AddKeyBatchIsAllOrNothing(t *testing.T) {
	cases := []struct {
		name       string
		batch      []KeyInput
		existing   []string
		wantCode   string
		wantStored int
	}{
		{
			name:       "a valid batch stores every row",
			batch:      []KeyInput{{Label: "x", Value: "sk-x"}, {Label: "y", Value: "sk-y"}, {Label: "z", Value: "sk-z"}},
			existing:   []string{"primary"},
			wantStored: 4,
		},
		{
			name:     "a duplicate label inside the batch writes nothing",
			batch:    []KeyInput{{Label: "dup", Value: "sk-a"}, {Label: "dup", Value: "sk-b"}},
			existing: []string{"primary"}, wantCode: "CONFLICT", wantStored: 1,
		},
		{
			name:     "a collision with an existing label writes nothing",
			batch:    []KeyInput{{Label: "fresh", Value: "sk-a"}, {Label: "primary", Value: "sk-b"}},
			existing: []string{"primary"}, wantCode: "CONFLICT", wantStored: 1,
		},
		{
			name:     "an empty value in the last row writes nothing",
			batch:    []KeyInput{{Label: "ok", Value: "sk-a"}, {Label: "bad", Value: "   "}},
			existing: []string{"primary"}, wantCode: "VALIDATION_ERROR", wantStored: 1,
		},
		{
			name:     "an over-long batch writes nothing",
			batch:    makeBatchKeys(101),
			existing: []string{"primary"}, wantCode: "VALIDATION_ERROR", wantStored: 1,
		},
		{
			name:     "an empty batch is refused",
			batch:    nil,
			existing: []string{"primary"}, wantCode: "VALIDATION_ERROR", wantStored: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			ctx := context.Background()
			endpoint := keyedEndpoint(t, svc, tc.existing...)

			added, err := svc.AddKeyBatch(ctx, endpoint.ID(), tc.batch)
			if tc.wantCode != "" {
				mustAppError(t, err, tc.wantCode)
				if len(added) != 0 {
					t.Fatalf("a refused batch returned %d rows, want none", len(added))
				}
			} else if err != nil {
				t.Fatalf("AddKeyBatch() error = %v", err)
			}

			current, getErr := svc.Get(ctx, endpoint.ID())
			if getErr != nil {
				t.Fatal(getErr)
			}
			if len(current.Keys()) != tc.wantStored {
				t.Fatalf("stored keys = %d, want %d: a refused batch must write nothing",
					len(current.Keys()), tc.wantStored)
			}
		})
	}
}

// TestEndpointService_AddKeyBatchAttributesTheBadRow pins §8.1's per-row report: the
// refusal names the index that caused it, so a client can show which row failed.
func TestEndpointService_AddKeyBatchAttributesTheBadRow(t *testing.T) {
	cases := []struct {
		name      string
		batch     []KeyInput
		wantIndex int
	}{
		{
			name:      "the offending row is the second",
			batch:     []KeyInput{{Label: "ok", Value: "sk-a"}, {Label: "", Value: "   "}},
			wantIndex: 1,
		},
		{
			name:      "the offending row is the first",
			batch:     []KeyInput{{Value: "   "}, {Label: "ok", Value: "sk-b"}},
			wantIndex: 0,
		},
		{
			name:      "the offending row is the last of four",
			batch:     []KeyInput{{Label: "a", Value: "sk-a"}, {Label: "b", Value: "sk-b"}, {Label: "c", Value: "sk-c"}, {Value: "   "}},
			wantIndex: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newEndpointSvc(t)
			endpoint := keyedEndpoint(t, svc, "primary")

			_, err := svc.AddKeyBatch(context.Background(), endpoint.ID(), tc.batch)
			mustAppError(t, err, "VALIDATION_ERROR")

			var indexed BulkRowIndexer
			if !errors.As(err, &indexed) {
				t.Fatalf("error %v does not name the offending row", err)
			}
			index, ok := indexed.BulkRowIndex()
			if !ok || index != tc.wantIndex {
				t.Fatalf("BulkRowIndex() = (%d, %v), want (%d, true)", index, ok, tc.wantIndex)
			}
		})
	}
}

// toKeyInputs turns labels into key inputs, so a table case can state just the labels.
func toKeyInputs(labels []string) []KeyInput {
	out := make([]KeyInput, 0, len(labels))
	for _, label := range labels {
		out = append(out, KeyInput{Label: label, Value: "sk-" + label})
	}
	return out
}

func makeBatchKeys(n int) []KeyInput {
	out := make([]KeyInput, 0, n)
	for i := range n {
		out = append(out, KeyInput{Label: "k" + strconv.Itoa(i), Value: "sk-value-" + strconv.Itoa(i)})
	}
	return out
}

func strPtr(value string) *string { return &value }

func intPtr(value int) *int { return &value }
