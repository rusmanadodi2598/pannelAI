// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/answer_toolnames_stream_test.go
// @for       The streamed half of the tool-name restore: a name split across chunks, the held-back suffix, and the wrapper's passthrough.
// @uses      strings, testing.
// @reason    A streamed chat delta may carry a tool call's name in fragments, so the restore has to rejoin them before it can match, and it must never hand the client a fragment a later chunk contradicts. The live failure was one name in one chunk; these tests pin the fragmented shape on top, because a naive per-chunk lookup restores nothing once the name is split.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-15
package dataplane

import (
	"strings"
	"testing"
)

// streamedNames drives the rewriter over a chunk sequence and returns the name
// a client would have read for each tool call, by call index, the way a client
// assembles deltas: every name fragment is appended in the order it arrived.
func streamedNames(t *testing.T, aliases map[string]string, chunks ...string) map[int]string {
	t.Helper()
	rewriter := newToolNameRewriter(aliases)
	names := make(map[int]string)
	for _, chunk := range chunks {
		payload, ok := decodeObject(rewriter.rewrite([]byte(chunk)))
		if !ok {
			t.Fatalf("payload did not decode: %s", chunk)
		}
		choices, _ := arrayField(payload, "choices")
		for _, choiceRaw := range choices {
			choice, _ := decodeObject(choiceRaw)
			delta, ok := objectField(choice, "delta")
			if !ok {
				continue
			}
			calls, _ := arrayField(delta, "tool_calls")
			for _, callRaw := range calls {
				call, _ := decodeObject(callRaw)
				function, ok := objectField(call, "function")
				if !ok {
					continue
				}
				names[intField(call, "index")] += stringField(function, "name")
			}
		}
	}
	return names
}

// streamChunk builds one chat delta carrying the given tool-call fragment.
func streamChunk(index int, fragment string) string {
	return `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":` +
		string(mustJSON(index)) + `,` + fragment + `}]}}]}`
}

// TestRestoreToolNamesStreamSplitAcrossChunks pins the accumulation rule: a
// name the upstream splits across chunks is rejoined before it is matched, so
// the caller reads back the name it declared rather than a fragment of the
// alias.
func TestRestoreToolNamesStreamSplitAcrossChunks(t *testing.T) {
	aliases := map[string]string{"read_client": "read"}
	chunks := []string{
		streamChunk(0, `"id":"c1","type":"function","function":{"name":"read_cli"}`),
		streamChunk(0, `"function":{"name":"ent"}`),
		streamChunk(0, `"function":{"arguments":"{\"path\":\"/tmp/x\"}"}`),
	}
	if got := streamedNames(t, aliases, chunks...)[0]; got != "read" {
		t.Fatalf("the client read name %q, want the caller's read", got)
	}
}

// TestRestoreToolNamesStreamWholeNameInOneChunk pins the shape the live
// failure arrived in: the whole alias in one delta is restored as it is.
func TestRestoreToolNamesStreamWholeNameInOneChunk(t *testing.T) {
	aliases := map[string]string{"shell_client": "shell"}
	chunks := []string{
		streamChunk(0, `"id":"c1","function":{"name":"shell_client","arguments":"{}"}`),
	}
	if got := streamedNames(t, aliases, chunks...)[0]; got != "shell" {
		t.Fatalf("the client read name %q, want the caller's shell", got)
	}
}

// TestRestoreToolNamesStreamPerCallIndex pins that fragments are accumulated
// per call, so two interleaved calls each read their own name.
func TestRestoreToolNamesStreamPerCallIndex(t *testing.T) {
	aliases := map[string]string{"read_client": "read", "shell_client": "shell"}
	chunks := []string{
		streamChunk(0, `"function":{"name":"read_cli"}`),
		streamChunk(1, `"function":{"name":"shell_cli"}`),
		streamChunk(0, `"function":{"name":"ent"}`),
		streamChunk(1, `"function":{"name":"ent"}`),
	}
	names := streamedNames(t, aliases, chunks...)
	if names[0] != "read" || names[1] != "shell" {
		t.Fatalf("names = %v, want call 0 read and call 1 shell", names)
	}
}

// TestRestoreToolNamesStreamHoldsBackWhatAnAliasCouldContradict pins the
// hold-back: a fragment that merely starts an alias is not yet the alias, so
// the part beyond a possible caller name waits, and a caller name that happens
// to be an alias prefix is completed by the chunk that proves it complete.
func TestRestoreToolNamesStreamHoldsBackWhatAnAliasCouldContradict(t *testing.T) {
	aliases := map[string]string{"read_client": "read"}

	// `read_cli` completes to the alias: the caller reads `read`.
	aliasSplit := streamedNames(t, aliases,
		streamChunk(0, `"function":{"name":"read_cli"}`),
		streamChunk(0, `"function":{"name":"ent","arguments":""}`),
	)
	if aliasSplit[0] != "read" {
		t.Fatalf("the split alias read %q, want read", aliasSplit[0])
	}

	// `read_cli` is the caller's own name: the arguments chunk closes it and
	// the held-back suffix arrives with it.
	callerName := streamedNames(t, aliases,
		streamChunk(0, `"function":{"name":"read_cli"}`),
		streamChunk(0, `"function":{"arguments":"{}"}`),
	)
	if callerName[0] != "read_cli" {
		t.Fatalf("the caller's own name read %q, want read_cli", callerName[0])
	}

	// A fragment past every alias can only be text: it is handed over at once.
	pastAlias := streamedNames(t, aliases,
		streamChunk(0, `"function":{"name":"read_client"}`),
		streamChunk(0, `"function":{"name":"X"}`),
	)
	if pastAlias[0] != "read_clientX" {
		t.Fatalf("the name past the alias read %q, want read_clientX", pastAlias[0])
	}
}

// TestRestoreToolNamesStreamPassthrough pins the wrapper's passthrough: a
// request that renamed nothing reaches the translator byte-identically, and a
// payload with no aliased name is handed on as it arrived.
func TestRestoreToolNamesStreamPassthrough(t *testing.T) {
	raw := []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"read_client"}}]}}]}`)

	plain := func(_ string, payload []byte) [][]byte { return [][]byte{payload} }
	frames := restoreToolNamesStream(nil, plain)(TargetOpenAI, raw)
	if len(frames) != 1 || string(frames[0]) != string(raw) {
		t.Fatalf("a request that renamed nothing must keep the payload byte-identical, got %s", frames)
	}

	// An alias map that names nothing in this payload is the same passthrough.
	frames = restoreToolNamesStream(map[string]string{"shell_client": "shell"}, plain)(TargetOpenAI, raw)
	if len(frames) != 1 || string(frames[0]) != string(raw) {
		t.Fatalf("a payload with no aliased name must keep its bytes, got %s", frames)
	}
}

// TestRestoreToolNamesStreamHandsTheTranslatorTheRestoredPayload pins the
// wrapper itself: the translator reads the restored payload, not the raw one.
func TestRestoreToolNamesStreamHandsTheTranslatorTheRestoredPayload(t *testing.T) {
	var seen []string
	translate := func(_ string, payload []byte) [][]byte {
		seen = append(seen, string(payload))
		return nil
	}
	chunk := `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"read_client","arguments":"{}"}}]}}]}`
	restoreToolNamesStream(map[string]string{"read_client": "read"}, translate)(TargetOpenAI, []byte(chunk))

	if len(seen) != 1 {
		t.Fatalf("translate ran %d times, want once", len(seen))
	}
	if !strings.Contains(seen[0], `"name":"read"`) || strings.Contains(seen[0], "read_client") {
		t.Fatalf("translate saw %s, want the caller's name", seen[0])
	}
}
