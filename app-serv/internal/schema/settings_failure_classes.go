// Package schema holds request/response DTOs and their validation rules.
//
// @file      internal/schema/settings_failure_classes.go
// @for       Lowering one wire table of per-class failure policies into the domain's, and rendering the stored table back.
// @uses      internal/domain.
// @reason    A class table is the one routing group keyed by a name the domain owns, so the boundary has to drop a class the router cannot produce rather than store an entry no failure will ever match. That rule is the whole reason this pair lives apart from the parking windows, which have no names to check.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     schema
// @stability stable
// @since     2026-10-15
package schema

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"

// failureClasses lowers one wire class table into the domain's, dropping a class
// the domain does not have rather than storing an entry no failure can match.
func failureClasses(in *map[string]FailureClassPolicyPatch) domain.FailureClassPolicies {
	if in == nil {
		return nil
	}
	out := make(domain.FailureClassPolicies, len(*in))
	for name, policy := range *in {
		class := domain.FailureClass(name)
		if !class.Recognised() {
			continue
		}
		lowered := domain.FailureClassPolicy{}
		if policy.Action != nil {
			lowered.Action = domain.FailureAction(*policy.Action)
		}
		if policy.CeilingSeconds != nil {
			lowered.CeilingSeconds = *policy.CeilingSeconds
		}
		if policy.CooldownSeconds != nil {
			lowered.CooldownSeconds = *policy.CooldownSeconds
		}
		out[class] = lowered
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// failureClassesResponse renders the stored entries, so a provider override reads
// back as the classes it names and nothing else. A table with no entry is nil
// rather than an empty map: the field is absent on the wire, not a {} an operator
// would read as "everything inherited" twice over.
func failureClassesResponse(in domain.FailureClassPolicies) map[string]FailureClassPolicyResponse {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]FailureClassPolicyResponse, len(in))
	for class, policy := range in {
		out[string(class)] = FailureClassPolicyResponse{
			Action:          string(policy.Action),
			CeilingSeconds:  policy.CeilingSeconds,
			CooldownSeconds: policy.CooldownSeconds,
		}
	}
	return out
}
