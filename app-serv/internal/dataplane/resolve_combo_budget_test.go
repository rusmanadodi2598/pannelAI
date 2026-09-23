// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/resolve_combo_budget_test.go
// @for       Draft 024 F1 after review: one resolution's work is bounded, so a
//
//	stored graph cannot turn a request into an unbounded number of
//	repository reads.
//
// @uses      internal/domain, context, fmt, testing.
// @reason    The depth guard alone bounded the chain length, not the work: a
//
//	combo whose members are combos re-walked every subtree once per path
//	to it, so a modest fan-out cost lookups exponentially. These tests
//	pin both bounds — the memo that collapses repeated subtrees, and the
//	expansion limit that stops a graph the memo cannot collapse — and the
//	alias-cycle guard the combo depth bound does not cover.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-24
package dataplane

import (
	"context"
	"fmt"
	"testing"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// countingLookup counts combo and alias reads, so a test can assert the work one
// resolution performs rather than only its answer.
type countingLookup struct {
	inner      ModelLookup
	comboReads int
	aliasReads int
}

func (l *countingLookup) Combo(ctx context.Context, name string) (domain.Combo, bool, error) {
	l.comboReads++
	return l.inner.Combo(ctx, name)
}

func (l *countingLookup) Alias(ctx context.Context, name string) (string, bool, error) {
	l.aliasReads++
	return l.inner.Alias(ctx, name)
}

func (l *countingLookup) Disabled(ctx context.Context, providerID, modelID string) (bool, error) {
	return l.inner.Disabled(ctx, providerID, modelID)
}

func (l *countingLookup) DisabledPairs(ctx context.Context) ([]domain.ModelRef, error) {
	return l.inner.DisabledPairs(ctx)
}

func (l *countingLookup) ComboNames(ctx context.Context) ([]string, error) {
	return l.inner.ComboNames(ctx)
}

// binaryTreeLookup builds a tree of combos where every internal node has two
// members naming the same two children, and every leaf fails to resolve. The
// shape is the worst case for a walker without a memo: the number of paths to
// the leaves doubles per level.
func binaryTreeLookup(depth int) fakeLookup {
	combos := map[string]domain.Combo{}
	for level := 0; level < depth; level++ {
		left, right := fmt.Sprintf("lvl%d-a", level+1), fmt.Sprintf("lvl%d-b", level+1)
		if level == depth-1 {
			// The leaves name a provider that is not in the index, so they fail.
			combos[fmt.Sprintf("lvl%d-a", level)] = comboRow(fmt.Sprintf("lvl%d-a", level), "nowhere/ghost", "nowhere/ghost")
			combos[fmt.Sprintf("lvl%d-b", level)] = comboRow(fmt.Sprintf("lvl%d-b", level), "nowhere/ghost", "nowhere/ghost")
			continue
		}
		combos[fmt.Sprintf("lvl%d-a", level)] = comboRow(fmt.Sprintf("lvl%d-a", level), left, right)
		combos[fmt.Sprintf("lvl%d-b", level)] = comboRow(fmt.Sprintf("lvl%d-b", level), left, right)
	}
	combos["root"] = comboRow("root", "lvl0-a", "lvl0-b")
	return fakeLookup{combos: combos}
}

// TestResolver_ComboWalkIsMemoized pins the work bound: a diamond of combos is
// walked once per distinct combo, not once per path through it. Without the memo
// the binary tree below costs 2^depth reads; with it, one per node.
func TestResolver_ComboWalkIsMemoized(t *testing.T) {
	const depth = 8
	lookup := &countingLookup{inner: binaryTreeLookup(depth)}
	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}
	if _, err := resolver.Resolve(context.Background(), "root"); err == nil {
		t.Fatal("Resolve() answered a tree whose leaves are all unresolvable")
	}
	// The walk is linear in the tree's depth: each level costs a constant number
	// of reads once the memo collapses the repeated subtrees (measured 31 for
	// depth 8, against 2^8 paths without it). The budget leaves room for the
	// entry point asking one name twice while failing the exponential shape.
	budget := 4*depth + 8
	if lookup.comboReads > budget {
		t.Fatalf("combo reads = %d, want at most %d for a %d-level tree", lookup.comboReads, budget, depth)
	}
}

// wideLookup builds a combo whose members are all distinct combos, each naming
// an unresolvable model: the shape a memo cannot collapse, so the expansion
// limit is what bounds it.
func wideLookup(width int) fakeLookup {
	combos := map[string]domain.Combo{}
	members := make([]string, 0, width)
	for i := 0; i < width; i++ {
		name := fmt.Sprintf("wide-%d", i)
		combos[name] = comboRow(name, "nowhere/ghost")
		members = append(members, name)
	}
	combos["wide-root"] = comboRow("wide-root", members...)
	return fakeLookup{combos: combos}
}

// TestResolver_ComboExpansionIsBounded pins the backstop: a graph wider than the
// expansion limit stops with MODEL_NOT_FOUND instead of walking it, so a stored
// configuration cannot make one request read the database without bound.
func TestResolver_ComboExpansionIsBounded(t *testing.T) {
	lookup := &countingLookup{inner: wideLookup(comboExpansionLimit * 2)}
	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}
	_, err = resolver.Resolve(context.Background(), "wide-root")
	if err == nil {
		t.Fatal("Resolve() answered a graph wider than the expansion limit")
	}
	if code := AsError(err).Code; code != CodeModelNotFound {
		t.Fatalf("code = %q, want %q", code, CodeModelNotFound)
	}
	// A member is read once by its parent and once by its own walk, so the read
	// count is bounded by twice the expansion limit. That is the property under
	// test: a stored graph cannot make one request read without bound.
	budget := 2*comboExpansionLimit + 4
	if lookup.comboReads > budget {
		t.Fatalf("combo reads = %d, want at most %d", lookup.comboReads, budget)
	}
}

// TestResolver_AliasCycleTerminates pins the one cycle shape the combo depth
// bound does not cover: an alias whose target is another alias. The write path
// refuses such a set, so this is the read path's behaviour for a row written
// directly — it must terminate rather than recurse.
func TestResolver_AliasCycleTerminates(t *testing.T) {
	lookup := fakeLookup{
		combos:  map[string]domain.Combo{},
		aliases: map[string]string{"x": "y", "y": "x"},
	}
	resolver, err := NewResolver(testIndex(t), lookup)
	if err != nil {
		t.Fatalf("building resolver: %v", err)
	}
	_, err = resolver.Resolve(context.Background(), "x")
	if err == nil {
		t.Fatal("Resolve() accepted an alias cycle")
	}
	if code := AsError(err).Code; code != CodeModelNotFound {
		t.Fatalf("code = %q, want %q", code, CodeModelNotFound)
	}
}
