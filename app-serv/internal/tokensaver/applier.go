// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/applier.go
// @for       Applying the enabled saver groups in their reference order, with
//
//	configuration read per request and every optional failure left open.
//
// @uses      internal/domain, context, encoding/json, errors, net/http.
// @reason    SPEC-API-002 §8 makes the saver pipeline a request-path concern but
//
// keeps each transform independently optional. This seam is where settings
// become decisions: RTK first, Headroom second, and Ponytail last. The
// caller only supplies the already translated upstream body, so no saver
// can accidentally run against the client wire and then be translated away.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// SettingsReader is the narrow settings port the pipeline reads. The service
// implementation owns persistence and defaults; the saver only needs the typed
// document.
type SettingsReader interface {
	Settings(context.Context) (domain.Settings, error)
}

// HeadroomTranslator pivots a non-OpenAI upstream body through the OpenAI
// message shape Headroom accepts, then puts the compressed messages back into
// the original wire. Prepare returns false when a body is not safe to send to
// the proxy, such as Responses input containing a reasoning or tool item.
type HeadroomTranslator interface {
	Prepare(body []byte, wire, model string) (HeadroomRequest, bool, error)
	Restore(body []byte, wire, model string, messages json.RawMessage) ([]byte, error)
}

// ApplierDeps holds the saver collaborators. Headroom is optional in tests and
// in deployments that have not configured an external proxy; the native RTK and
// Ponytail groups still work without it.
type ApplierDeps struct {
	Settings   SettingsReader
	Headroom   *HeadroomClient
	Translator HeadroomTranslator
}

// Applier applies the token-saver pipeline to one already translated upstream
// body. Its methods have no HTTP dependency, so the engine can use the seam for
// both streamed and non-streamed calls.
type Applier struct {
	settings   SettingsReader
	headroom   *HeadroomClient
	translator HeadroomTranslator
}

// NewApplier validates the required settings port and returns an applier. A
// missing translator falls back to the native OpenAI shape; the composition root
// supplies the dataplane translator for Claude and Responses targets.
func NewApplier(deps ApplierDeps) (*Applier, error) {
	if deps.Settings == nil {
		return nil, errors.New("tokensaver: settings reader is required")
	}
	translator := deps.Translator
	if translator == nil {
		translator = rawHeadroomTranslator{}
	}
	return &Applier{
		settings: deps.Settings, headroom: deps.Headroom, translator: translator,
	}, nil
}

// Apply runs RTK, Headroom, and Ponytail in the reference order. A bypass header,
// a settings read failure, a proxy failure, or a malformed optional transform
// all return the latest body rather than failing an otherwise valid upstream
// call.
func (a *Applier) Apply(ctx context.Context, body []byte, wire, model string, bypass bool) []byte {
	if a == nil || bypass || len(body) == 0 {
		return body
	}
	settings, err := a.settings.Settings(ctx)
	if err != nil {
		return body
	}
	if settings.TokenSaver.RTK.Enabled {
		if rewritten, _, compressErr := Compress(body, settings.TokenSaver.RTK.Filters); compressErr == nil {
			body = rewritten
		}
	}
	if settings.TokenSaver.Headroom.Enabled && a.headroom != nil && settings.TokenSaver.Headroom.URL != "" {
		body = a.applyHeadroom(ctx, body, wire, model, settings.TokenSaver.Headroom)
	}
	if settings.TokenSaver.Ponytail.Enabled {
		body = InjectPonytail(body, wire, settings.TokenSaver.Ponytail.Level)
	}
	return body
}

// applyHeadroom makes one optional proxy call and restores the compressed array
// only after the proxy has returned a shape the translator accepts.
func (a *Applier) applyHeadroom(
	ctx context.Context,
	body []byte,
	wire string,
	model string,
	settings domain.TokenSaverHeadroom,
) []byte {
	request, ok, err := a.translator.Prepare(body, wire, model)
	if err != nil || !ok {
		return body
	}
	request.URL = settings.URL
	request.CompressUserMessages = settings.CompressUserMessages
	result, err := a.headroom.Compress(ctx, request)
	if err != nil {
		return body
	}
	restored, err := a.translator.Restore(body, wire, model, result.Messages)
	if err != nil {
		return body
	}
	return restored
}

// rawHeadroomTranslator handles the native OpenAI body without a dataplane
// dependency. The composition root replaces it with the full translator for
// Claude and Responses upstreams.
type rawHeadroomTranslator struct{}

// Prepare returns the OpenAI messages member when the body is a chat envelope.
func (rawHeadroomTranslator) Prepare(body []byte, wire, model string) (HeadroomRequest, bool, error) {
	if wire != WireOpenAI {
		return HeadroomRequest{}, false, nil
	}
	envelope, ok := decodeObject(body)
	if !ok {
		return HeadroomRequest{}, false, nil
	}
	messages, ok := envelope["messages"]
	if !ok {
		return HeadroomRequest{}, false, nil
	}
	if _, ok := decodeArray(messages); !ok {
		return HeadroomRequest{}, false, nil
	}
	return HeadroomRequest{Messages: messages, Model: model}, true, nil
}

// Restore replaces only the OpenAI messages member, preserving sibling fields.
func (rawHeadroomTranslator) Restore(body []byte, wire, _ string, messages json.RawMessage) ([]byte, error) {
	if wire != WireOpenAI {
		return body, nil
	}
	envelope, ok := decodeObject(body)
	if !ok {
		return body, errors.New("tokensaver: the OpenAI body is not an object")
	}
	if _, ok := decodeArray(messages); !ok {
		return body, errors.New("tokensaver: Headroom returned no message array")
	}
	envelope["messages"] = messages
	encoded, err := marshalObject(envelope)
	if err != nil {
		return body, err
	}
	return encoded, nil
}

// NewApplierWithClient is the composition-root convenience for the normal
// deployment: it creates the Headroom client over the already guarded HTTP
// client and keeps the full translator choice explicit at the call site.
func NewApplierWithClient(settings SettingsReader, client *http.Client, translator HeadroomTranslator) (*Applier, error) {
	return NewApplier(ApplierDeps{
		Settings: settings, Headroom: NewHeadroomClient(client), Translator: translator,
	})
}
