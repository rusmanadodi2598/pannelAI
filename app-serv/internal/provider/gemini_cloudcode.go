// Package provider holds the per-provider plugin seam: the connector that decides
// how one provider is authenticated and addressed.
//
// @file      internal/provider/gemini_cloudcode.go
// @for       The Cloud Code Assist families of the Gemini wire: the project envelope, the model-less endpoint, and the client identification the vendor reads.
// @uses      crypto/sha256, encoding/json, fmt, net/http, strings, time, internal/registry.
// @reason    Gemini CLI and Antigravity both send the generateContent payload this gateway already translates, but address an endpoint that names no model and read a body whose payload sits inside an envelope keyed to the account's Cloud project. Two families share that shape and differ only in the attribution members one adds, so one connector with a small product table replaces branches in shared code.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-18
package provider

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"
)

// CloudCode is the connector for the Gemini families reached through Cloud Code
// Assist. It reuses the plain Gemini connector for credential placement and adds
// the two things this endpoint expects: an address with no model segment, and the
// envelope around the payload.
type CloudCode struct {
	*Gemini
	entry   registry.Provider
	product CloudCodeProduct
}

// CloudCodeProduct names the members one Cloud Code Assist product sends beside the
// payload, and the client identification it sends with the request.
type CloudCodeProduct struct {
	// attribution marks the product that sends userAgent and requestId, which the
	// endpoint reads as its own session accounting.
	attribution bool
	// userAgentValue is that member's literal value.
	userAgentValue string
	// clientHeader names the request header carrying the entry's declared SDK
	// client string, empty for a product that identifies itself by user agent only.
	clientHeader string
	// agentPrefix builds the request User-Agent from the entry's declared CLI
	// version, empty for a product whose entry already declares its own header.
	agentPrefix string
}

// The two Cloud Code Assist products this port serves.
var (
	CloudCodeGeminiCLI = CloudCodeProduct{
		clientHeader: "X-Goog-Api-Client",
		agentPrefix:  "gemini-cli/",
	}
	CloudCodeAntigravity = CloudCodeProduct{
		attribution:    true,
		userAgentValue: "antigravity",
	}
)

// NewCloudCode builds the connector for one Cloud Code Assist registry entry.
func NewCloudCode(entry registry.Provider, product CloudCodeProduct) *CloudCode {
	return &CloudCode{Gemini: NewGemini(entry), entry: entry, product: product}
}

const (
	cloudcodeAction       = ":generateContent"
	cloudcodeStreamAction = ":streamGenerateContent?alt=sse"
)

// Endpoint implements provider.Plugin. The action goes on the entry's base and the
// model travels in the body, which is the opposite of the plain generateContent
// endpoint: the account's project, not a path segment, decides what is addressed.
func (c *CloudCode) Endpoint(req Request, _ Credential) (string, error) {
	base, err := c.Default.Endpoint(req, Credential{})
	if err != nil {
		return "", err
	}
	if req.Stream {
		return base + cloudcodeStreamAction, nil
	}
	return base + cloudcodeAction, nil
}

// TransformRequest implements provider.Transformer: it wraps the translated Gemini
// payload in the envelope this endpoint unwraps before it looks at a model.
//
// A project the account does not store is left out rather than invented: a value the
// vendor never granted would attribute the call to something that does not exist,
// and the refusal that absence earns is a signal an operator can act on.
func (c *CloudCode) TransformRequest(req *Request) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		return fmt.Errorf("provider %s: the translated payload could not be read", req.Provider.ID)
	}

	envelope := cloudcodeEnvelope{
		Model:   cloudcodeWireModel(req.Model.UpstreamID()),
		Request: req.Body,
		Project: strings.TrimSpace(req.Credential.MetadataValue(MetadataProjectID)),
	}
	if c.product.attribution {
		userAgent := c.product.userAgentValue
		envelope.UserAgent = &userAgent
		envelope.RequestID = c.requestID(req, cloudcodeTurns(payload["contents"]))
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("provider %s: the request envelope could not be encoded", req.Provider.ID)
	}
	req.Body = encoded
	return nil
}

// ApplyAuth implements provider.Plugin: the entry's credential placement first, then
// the client identification this endpoint reads. Both members have been declared in
// the registry document since it was generated and had no reader until here.
func (c *CloudCode) ApplyAuth(request *http.Request, credential Credential) error {
	if err := c.Gemini.ApplyAuth(request, credential); err != nil {
		return err
	}
	if c.product.clientHeader != "" {
		if client := strings.TrimSpace(c.entry.Transport.APIClient); client != "" {
			request.Header.Set(c.product.clientHeader, client)
		}
	}
	if c.product.agentPrefix != "" {
		if version := strings.TrimSpace(c.entry.Transport.CLIVersion); version != "" {
			request.Header.Set("User-Agent", c.product.agentPrefix+version)
		}
	}
	return nil
}

// cloudcodeEnvelope is the body this endpoint reads. The attribution members are
// pointers or omitempty so a product that sends them keeps their names and the other
// sends nothing at all.
type cloudcodeEnvelope struct {
	Project   string          `json:"project,omitempty"`
	Model     string          `json:"model"`
	UserAgent *string         `json:"userAgent,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
	Request   json.RawMessage `json:"request"`
}

// requestID builds the id the IDE product attributes a turn by: a conversation and a
// trajectory derived from the account and the model, the instant of the call, and a
// step the conversation length decides.
//
// Both ids are seeded rather than random, so a retry of one turn arrives under the
// identity it was first counted with. The step counts two per turn (a user ask and a
// model answer) and never below one.
func (c *CloudCode) requestID(req *Request, turns int) string {
	// The project is the session this product attributes by, so two projects of one
	// account do not blend into one conversation and a retry of one project keeps it.
	session := req.Credential.MetadataValue(MetadataProjectID)
	if session == "" {
		session = req.Credential.Account()
	}
	if session == "" {
		session = req.Credential.EndpointID()
	}
	product := c.product.userAgentValue
	conversation := cloudcodeSeedID(product + ":conversation:" + session)
	trajectory := cloudcodeSeedID(product + ":trajectory:" + session + ":" + cloudcodeWireModel(req.Model.UpstreamID()))

	step := 1
	if turns > 0 {
		step = turns*2 - 1
	}
	return fmt.Sprintf("agent/%s/%d/%s/%d", conversation, time.Now().UnixMilli(), trajectory, step)
}

// cloudcodeWireModel drops the trailing thinking level from a declared model id.
// The catalog writes `gemini-3.8-flash-high(high)` because that is how the level is
// named on the way in, and the level has already been applied to the request's
// thinking budget; Cloud Code Assist answers 404 for a model string that still
// carries the parenthetical.
func cloudcodeWireModel(model string) string {
	model = strings.TrimSpace(model)
	open := strings.LastIndex(model, "(")
	if open <= 0 || !strings.HasSuffix(model, ")") {
		return model
	}
	return strings.TrimSpace(model[:open])
}

// cloudcodeTurns counts the turns the translated payload carries. The count comes
// from the payload being wrapped rather than a re-read of the body: at this point
// the envelope does not exist yet, and the bytes that will become its `request`
// member are the only conversation the product can see.
func cloudcodeTurns(raw json.RawMessage) int {
	var contents []json.RawMessage
	if err := json.Unmarshal(raw, &contents); err != nil {
		return 0
	}
	return len(contents)
}

// cloudcodeSeedID renders a seed string as the uuid shape the endpoint parses. The
// hash is an identity rather than a credential, so sha256 is enough and it stays
// reproducible across processes.
func cloudcodeSeedID(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	hex := fmt.Sprintf("%x", sum[:16])
	return hex[:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}
