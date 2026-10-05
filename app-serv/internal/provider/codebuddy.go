// Package provider ports the reference's per-provider request shaping into
// pluggable connectors (SPEC-API-001 §7.4).
//
// @file      internal/provider/codebuddy.go
// @for       The CodeBuddy connector: the registry entry served, apart from the two
//
//	things this vendor's endpoint does not accept as they arrive.
//
// @uses      internal/registry.
// @reason    The reference forces every CodeBuddy request onto a stream at
//
//	`chatCore.js:133`, reading `forceStream: true` off the provider entry, and its
//	executor rebuilds the message list because the vendor answers a plain OpenAI body
//	with `11101`. This port decides forced streaming from the connector instead, a
//	rule pinned by TestTransport_ForcesStreamIsDeclaredByTheConnector, so a provider
//	that needs it has to say so here. Without this file the gateway sends
//	`stream:false` to an SSE-only service whenever the client asked for one JSON body,
//	and the body it sends is one the vendor refuses to read.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-09-29
package provider

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"

// CodeBuddy is the connector for the CodeBuddy regions. It exists for two things the
// plain OpenAI fallback cannot give this vendor: every request has to be a stream, and
// the message list has to be shaped the way the service reads it (codebuddy_body.go).
// The URL, headers and bearer credential stay the fallback's work.
type CodeBuddy struct {
	*Default
}

// NewCodeBuddy builds the connector for one CodeBuddy registry entry.
func NewCodeBuddy(entry registry.Provider) *CodeBuddy {
	return &CodeBuddy{Default: NewDefault(entry)}
}

// ForcesStream implements provider.StreamForcer: this service answers a chat
// request as a server-sent stream and the reference treats it as a hard
// requirement rather than a preference, so a client that asked for one JSON body
// is served from a stream the gateway folds back, the same answer, arriving the
// way the vendor will actually send it.
func (c *CodeBuddy) ForcesStream() bool { return true }
