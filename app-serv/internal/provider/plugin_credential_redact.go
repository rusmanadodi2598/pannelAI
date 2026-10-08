// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/plugin_credential_redact.go
// @for       Keeping one account's credential material out of whatever prints or logs the struct.
// @uses      fmt, log/slog, strings.
// @reason    A credential is the one value in the seam whose disclosure is an incident, and the surfaces that render a struct are more numerous than the ones that read it: a debug verb, a test failure, a log line someone adds beside a call site that already holds the value.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-08
package provider

import (
	"fmt"
	"log/slog"
	"strings"
)

// GoString is the verb String() cannot reach. fmt formats a struct for %#v by
// reflecting over its fields, which prints a nested field's value and never asks the
// field's own String method, so `%#v` on a dataplane.Call or a Selection answers with
// the plaintext credential while `%v` on the same value is already redacted.
func (c Credential) GoString() string {
	return fmt.Sprintf("provider.Credential{endpointID: %q, keyID: %q, family: %q, account: %q, "+
		"projectID: %q, apiKey: %q, accessToken: %q}",
		c.endpointID, c.keyID, c.family.String(), c.account, c.projectID,
		redactedWhenSet(c.apiKey), redactedWhenSet(c.accessToken))
}

// LogValue is how a slog record names the account without taking the credential with
// it. Without this slog reflects the struct, finds only unexported fields, and writes
// an empty object: the log line then carries neither a leak nor any idea of which
// account the call was for.
func (c Credential) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("endpoint_id", c.endpointID),
		slog.String("key_id", c.keyID),
		slog.String("account", c.account),
		slog.String("api_key", redactedWhenSet(c.apiKey)),
		slog.String("access_token", redactedWhenSet(c.accessToken)),
	)
}

func redactedWhenSet(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unset"
	}
	return "[redacted]"
}
