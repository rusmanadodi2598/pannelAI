// Package domain holds entities, value objects, and the ubiquitous language
// of the pannelAI gateway (SPEC-API-001 §5).
//
// @file      internal/domain/health.go
// @for       The dependency health vocabulary shared by service and handler.
// @uses      standard library only.
// @reason    SPEC-API-001 §7.1 has health report dependency reachability; the
//
//	vocabulary belongs to the domain so the service never depends on
//	the schema (DTO) layer (AGENTS.md §1.5 layer flow).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     domain
// @stability experimental
// @since     2026-09-16
package domain

// HealthState is the aggregate liveness of app-serv dependencies.
type HealthState string

const (
	// HealthUP means every configured dependency answered its probe.
	HealthUP HealthState = "ok"

	// HealthDown means at least one dependency did not answer.
	HealthDown HealthState = "degraded"
)

// ProbeState is one dependency's outcome.
type ProbeState string

const (
	// ProbeOK means the dependency answered.
	ProbeOK ProbeState = "ok"

	// ProbeDown means the dependency did not answer. The driver's error is
	// logged, never returned: §7.1 makes health a public endpoint, and a
	// connection error names the database and role it failed to reach.
	ProbeDown ProbeState = "down"

	// ProbeSkipped marks a dependency that was not configured, which keeps a
	// partially configured dev server usable instead of reporting it unhealthy.
	ProbeSkipped ProbeState = "skipped"
)

// ProbeResult is one dependency's outcome, reported verbatim to the caller.
type ProbeResult struct {
	Name  string
	State ProbeState
}
