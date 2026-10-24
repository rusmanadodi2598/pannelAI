// Package provider holds the per-provider plugin seam: the connector that decides
// how one provider is authenticated and addressed.
//
// @file      internal/provider/gemini.go
// @for       The Gemini generateContent connector: the endpoint each call is sent to.
// @uses      fmt, strings, internal/registry.
// @reason    This wire names the model in the request path and switches between a JSON answer and a server-sent stream by choosing a different action there, which the OpenAI fallback cannot express: it joins a base with a declared path only, and never reads the model. Authentication and headers stay the fallback's work because the entry already declares them per credential family.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-17
package provider

import (
	"fmt"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// Gemini is the connector for providers on the generateContent wire. It exists for
// the URL alone: the model is a path segment and the action depends on whether the
// caller asked for a stream.
type Gemini struct {
	*Default
}

// NewGemini builds the connector for one Gemini registry entry.
func NewGemini(entry registry.Provider) *Gemini {
	return &Gemini{Default: NewDefault(entry)}
}

const (
	geminiAction       = "generateContent"
	geminiStreamAction = "streamGenerateContent?alt=sse"
)

// Endpoint implements provider.Plugin: {base}/{model}:generateContent, or the
// streaming action with alt=sse, which is how this wire asks for SSE rather than a
// body member named "stream" the vendor does not read.
//
// The model must be one path segment. A value carrying a separator, a query, a
// fragment, or a colon would pick a different endpoint, or a different action, than
// the entry declares, and that is a choice about the destination rather than about
// spelling.
func (g *Gemini) Endpoint(req Request, cred Credential) (string, error) {
	base, err := g.Default.Endpoint(req, cred)
	if err != nil {
		return "", err
	}
	model := strings.TrimSpace(req.Model.UpstreamID())
	if !isGeminiPathSegment(model) {
		return "", fmt.Errorf("provider %s: model %q cannot be addressed as one path segment", req.Provider.ID, model)
	}
	action := geminiAction
	if req.Stream {
		action = geminiStreamAction
	}
	return joinPath(base, model+":"+action), nil
}

// isGeminiPathSegment reports whether a model id is safe to put in the path.
func isGeminiPathSegment(model string) bool {
	if model == "" {
		return false
	}
	return !strings.ContainsAny(model, "/?#: \\")
}
