// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/opencode_request.go
// @for       The per-turn request identity the OpenCode CLI carries in x-opencode-request.
// @uses      crypto/rand, crypto/sha256, encoding/hex, regexp, time, net/http, internal/provider/opencode_body.go helpers.
// @reason    The CLI names the user message it is sending, and OpenCode 1.19.0 reads that name's twelve hex characters back as a timestamp, so an id that only looks canonical is refused. Deriving the id from the session plus the last user message keeps one turn on one id, which is what makes a retry a retry instead of a second charge against the free tier.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability stable
// @since     2026-10-14
package provider

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// openCodeRequestHeader is the identity header the CLI sends for the current
// turn. It sits here rather than in opencode.go, which is at the review line
// budget already.
const openCodeRequestHeader = "x-opencode-request"

// openCodeRequestCachePrefix namespaces remembered request ids away from
// remembered session ids.
const openCodeRequestCachePrefix = "opencode:rq:"

// openCodeIDSuffixLen is the base62 tail of an OpenCode id, and openCodeIDTail
// is the reference's cap on the message text the id is derived from.
const (
	openCodeIDSuffixLen = 14
	openCodeIDTail      = 600
)

// OpenCodeRequestRE is the request-id shape the upstream accepts, the mirror of
// OpenCodeSessionRE with the message prefix.
var OpenCodeRequestRE = regexp.MustCompile(`^msg_[0-9a-f]{12}[0-9A-Za-z]{14}$`)

// openCodeRequests remembers one id per (session, last user message).
var openCodeRequests = newOpenCodeIDCache()

// OpenCodeRequestID names the turn a request carries. It reads the outbound body
// after the connector has shaped it, so the text it hashes is the text that
// leaves the process and a retry of the same turn derives the same id.
//
// A body that cannot be read, or one with no user text to name, falls back to a
// freshly minted id rather than failing the request: this header is an identity,
// not a precondition, and refusing here would turn a upstream refusal into a
// gateway one.
func OpenCodeRequestID(sessionID string, req *http.Request) string {
	body, err := readForSigning(req)
	if err != nil || len(body) == 0 {
		return openCodeRandomRequestID()
	}
	text := lastOpenCodeUserText(body)
	if text == "" {
		return openCodeRandomRequestID()
	}
	digest := sha256.Sum256([]byte("opencode-req\x00" + sessionID + "\x00" + text))
	key := openCodeRequestCachePrefix + sessionID + ":" + hex.EncodeToString(digest[:8])
	return openCodeRequests.getOrMint(key, func(now time.Time, _ uint64) string {
		return "msg_" + openCodeAscendingHex(now.UnixMilli()) + base62(digest[6:20])
	})
}

// openCodeRandomRequestID mints an id for a turn with nothing to name, which is
// what the reference's generateRequestId does with its unstable tail.
func openCodeRandomRequestID() string {
	return "msg_" + openCodeAscendingHex(time.Now().UnixMilli()) + openCodeUnstableSuffix()
}

// openCodeUnstableSuffix is the reference's unstableRandom(): a tail that differs
// per call. A system whose entropy source fails is not a case worth a second
// identity scheme, so the clock stands in for the draw.
func openCodeUnstableSuffix() string {
	buf := make([]byte, openCodeIDSuffixLen)
	if _, err := rand.Read(buf); err != nil {
		fallback := sha256.Sum256([]byte(fmt.Sprint(time.Now().UnixNano())))
		return base62(fallback[:openCodeIDSuffixLen])
	}
	return base62(buf)
}

// lastOpenCodeUserText reads the text that names the current turn, which is the
// last user message of the conversation. `messages` is read before `input`
// exactly as the reference does, so a body carrying both is named the same way
// the CLI names it, and a bare string `input` is a text worth naming on its own.
func lastOpenCodeUserText(body []byte) string {
	decoded, ok := decodeOpenCodeBody(body)
	if !ok {
		return ""
	}
	items, isArray := decodeOpenCodeItems(decoded["messages"])
	if !isArray {
		items, isArray = decodeOpenCodeItems(decoded["input"])
	}
	if !isArray {
		return openCodeTail(stringMember(decoded, "input"))
	}
	for index := len(items) - 1; index >= 0; index-- {
		item, ok := decodeOpenCodeBody(items[index])
		if !ok {
			continue
		}
		if role := stringMember(item, "role"); role != "" && role != "user" {
			continue
		}
		if text := openCodeContentText(item["content"]); text != "" {
			return openCodeTail(text)
		}
	}
	return ""
}

// openCodeContentText reads a message's text whether the content is a plain
// string or a list of parts. Parts are joined with one space and a part is read
// as `text` before `input_text`, which is the reference's order.
func openCodeContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return strings.TrimSpace(plain)
	}
	parts, ok := decodeOpenCodeItems(raw)
	if !ok {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		var asText string
		if err := json.Unmarshal(part, &asText); err == nil {
			texts = append(texts, asText)
			continue
		}
		entry, ok := decodeOpenCodeBody(part)
		if !ok {
			continue
		}
		if value := stringMember(entry, "text"); value != "" {
			texts = append(texts, value)
		} else if value := stringMember(entry, "input_text"); value != "" {
			texts = append(texts, value)
		}
	}
	return strings.TrimSpace(strings.Join(texts, " "))
}

// openCodeTail keeps the last openCodeIDTail runes of a text. The reference
// slices characters, and slicing bytes here would split a rune between two
// retries of the same message and hand them different ids.
func openCodeTail(text string) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= openCodeIDTail {
		return text
	}
	return string(runes[len(runes)-openCodeIDTail:])
}
