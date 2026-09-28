// Package registry holds the embedded provider registry: the static provider
// catalog loaded once at boot.
//
// @file      internal/registry/types_retry.go
// @for       The retry override a transport declares: how many attempts and,
//
//	for a provider whose transient recovers over seconds, how far apart.
//
// @uses      fmt, strconv, time, gopkg.in/yaml.v3.
// @reason    The reference writes retry as a bare count, a per-status count, and
//
//	a per-status object, so the union of those shapes is decoded here rather than
//	loosened into a map that would let a misspelled key silently disable a retry.
//	The backoff base is separated from the attempt count because a provider whose
//	upstream answers the same request "all backends failed" and serves it seconds
//	later needs the retries spaced, not merely more of them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-28
package registry

import (
	"fmt"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Retry is a provider's attempt override. The reference declares three shapes
// for one field: a count, a per-status count, and a per-status object. Both
// are normalised here, because a decoder that accepted only one would silently
// drop the others.
type Retry struct {
	DefaultAttempts int

	// ByStatus overrides DefaultAttempts for one upstream status code.
	ByStatus map[int]int

	// BackoffBaseMS overrides the gateway's default backoff base for one status.
	// It exists for a provider whose transient refusal recovers over seconds, not
	// milliseconds: Qoder's free-model pool answers the identical request "all
	// backends failed" / "quota exceeded" and serves it seconds later (draft 036
	// §9.2), so a sub-second retry re-fires inside the same fail-streak. An entry
	// that declares no base keeps the gateway default, so this changes no other
	// provider.
	BackoffBaseMS map[int]int
}

// Attempts reports how many times to retry a given upstream status.
func (r Retry) Attempts(status int) int {
	if n, ok := r.ByStatus[status]; ok {
		return n
	}
	return r.DefaultAttempts
}

// BackoffBase reports the seconds-scale spacing an entry asked for one status,
// and whether it asked at all. The second return is the point: an entry that
// declares no base must retry on the gateway default, not on a zero wait.
func (r Retry) BackoffBase(status int) (time.Duration, bool) {
	ms, ok := r.BackoffBaseMS[status]
	if !ok || ms <= 0 {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}

// UnmarshalYAML accepts the scalar and mapping shapes of the retry field.
func (r *Retry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		n, err := strconv.Atoi(node.Value)
		if err != nil {
			return fmt.Errorf("registry: retry must be a number or a map: %q", node.Value)
		}
		r.DefaultAttempts = n
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("registry: retry must be a number or a map")
	}

	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return err
	}
	r.ByStatus = make(map[int]int, len(raw))
	r.BackoffBaseMS = make(map[int]int, len(raw))
	for key, value := range raw {
		status, err := strconv.Atoi(key)
		if err != nil {
			return fmt.Errorf("registry: retry key %q is not a status code", key)
		}
		attempts, backoffMS, err := retryRule(value)
		if err != nil {
			return err
		}
		r.ByStatus[status] = attempts
		if backoffMS > 0 {
			r.BackoffBaseMS[status] = backoffMS
		}
	}
	return nil
}

// retryRule reads either the count form or the {attempts, backoff_ms} object.
// A bare count carries no backoff, and an object may state either member alone.
func retryRule(node yaml.Node) (attempts, backoffMS int, err error) {
	if node.Kind == yaml.ScalarNode {
		n, err := strconv.Atoi(node.Value)
		if err != nil {
			return 0, 0, fmt.Errorf("registry: retry value %q is not a number", node.Value)
		}
		return n, 0, nil
	}
	var shaped struct {
		Attempts  int `yaml:"attempts"`
		BackoffMS int `yaml:"backoff_ms"`
	}
	if err := node.Decode(&shaped); err != nil {
		return 0, 0, err
	}
	return shaped.Attempts, shaped.BackoffMS, nil
}
