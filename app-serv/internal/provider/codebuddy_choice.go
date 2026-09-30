// The CodeBuddy forced tool choice: this vendor reads the OpenAI wire's
// "call a tool" as `required` and has no shape for "call this one", so the
// "this one" half is carried by handing it a single tool.
//
// @file      internal/provider/codebuddy_choice.go
// @for       Rewriting a tool_choice that names one function into the request this
//
//	vendor answers.
//
// @uses      encoding/json, fmt.
// @reason    Measured live on codebuddy-intl/deepseek-v4.1-flash on 2026-09-30:
//
//	`tool_choice:"auto"` and `"required"` are answered, and the OpenAI
//	named-function object draws `Bad Request`. The named-function form is
//	what an agent sends when it has already decided which tool must run,
//	so refusing it would drop the request on a vendor that can be given
//	what it lacks — narrowing the tool list to that one function makes
//	`required` mean "call this one", because nothing else is callable.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-30
package provider

import (
	"encoding/json"
	"fmt"
)

// namedToolChoice is the OpenAI tool_choice that names one function: the shape a
// caller sends when it has already chosen the tool.
type namedToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// mirrorCodeBuddyToolChoice rewrites a forced function into a shape this vendor
// answers. A choice it already accepts — auto, none, required, or absent — is left
// exactly as the client sent it.
func mirrorCodeBuddyToolChoice(body map[string]json.RawMessage) error {
	raw, present := body["tool_choice"]
	if !present {
		return nil
	}
	named, ok := decodeNamedToolChoice(raw)
	if !ok {
		return nil
	}
	tools, found, err := onlyNamedTool(body["tools"], named.Function.Name)
	if err != nil {
		return err
	}
	if !found {
		// A choice naming a tool the request never declared is the caller's
		// mistake, and this connector does not answer it by inventing a tool or
		// by silently letting the model pick another one. The request goes on as
		// sent, and the vendor refuses it as it refuses the shape.
		return nil
	}
	body["tools"] = tools
	body["tool_choice"] = json.RawMessage(`"required"`)
	return nil
}

// decodeNamedToolChoice reports whether a tool_choice value is the object naming
// one function, as opposed to a string the vendor already reads.
func decodeNamedToolChoice(raw json.RawMessage) (namedToolChoice, bool) {
	var named namedToolChoice
	if err := json.Unmarshal(raw, &named); err != nil {
		return namedToolChoice{}, false
	}
	if named.Type != "function" || named.Function.Name == "" {
		return namedToolChoice{}, false
	}
	return named, true
}

// onlyNamedTool returns the declared tools reduced to the one the caller forced,
// reporting false when the request declares no such tool.
func onlyNamedTool(raw json.RawMessage, name string) (json.RawMessage, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	var declared []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &declared); err != nil {
		return nil, false, fmt.Errorf("tools must be a list of tool objects")
	}
	for _, tool := range declared {
		if toolFunctionName(tool) != name {
			continue
		}
		kept, err := json.Marshal([]map[string]json.RawMessage{tool})
		if err != nil {
			return nil, false, fmt.Errorf("the forced tool could not be encoded: %w", err)
		}
		return kept, true, nil
	}
	return nil, false, nil
}

// toolFunctionName reads the name out of one declared tool, which OpenAI's wire
// nests under a `function` member.
func toolFunctionName(tool map[string]json.RawMessage) string {
	function, present := tool["function"]
	if !present {
		return ""
	}
	var named struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(function, &named); err != nil {
		return ""
	}
	return named.Name
}
