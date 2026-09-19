// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_build.go
// @for       The build-output filter: a build log reduced to its errors, its
//
//	warnings, and its summary.
//
// @uses      fmt, regexp, strings.
// @reason    SPEC-API-002 §5 ports the reference's build-output filter, which
//
//	covers npm, yarn, cargo, and the JVM builders. A build log is read
//	for what failed, and the progress lines in between are the bulk of
//	it.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reBuildCargoContinuation = regexp.MustCompile(`^\s*(-->|\||\d+\s*\||=)`)
	reBuildNpmError          = regexp.MustCompile(`^npm (ERR!|error)`)
	reBuildYarnError         = regexp.MustCompile(`^yarn error`)
	reBuildDeprecation       = regexp.MustCompile(`^npm warn deprecated`)
	reBuildNpmWarn           = regexp.MustCompile(`^npm warn`)
	reBuildYarnWarn          = regexp.MustCompile(`^yarn warn`)
	reBuildErrorHeading      = regexp.MustCompile(`(?i)^error(\[|:)`)
	reBuildWarnHeading       = regexp.MustCompile(`(?i)^warning(\[|:)`)
	reBuildMavenError        = regexp.MustCompile(`^ERROR:`)
	reBuildBracketError      = regexp.MustCompile(`^\[ERROR\]`)
	reBuildFailed            = regexp.MustCompile(`^BUILD FAILED`)
	reBuildBracketWarn       = regexp.MustCompile(`^\[WARNING\]`)
	reBuildCompiling         = regexp.MustCompile(`^\s*Compiling\s+\S+`)
	reBuildDownloading       = regexp.MustCompile(`^\s*Downloading\s+\S+`)
	reBuildFetching          = regexp.MustCompile(`^Fetching\s+`)
	reBuildSummary           = regexp.MustCompile(`(?i)^(added|removed|changed|audited|installed)\s+\d+\s+package`)
	reBuildFinished          = regexp.MustCompile(`^\s*Finished\s+`)
	reBuildSuccess           = regexp.MustCompile(`^BUILD SUCCESS`)
	reBuildCounts            = regexp.MustCompile(`^\d+\s+(vulnerabilities|packages?|warnings?|errors?)`)
	reBuildInstalled         = regexp.MustCompile(`^Successfully (installed|built)`)
	reBuildAddressIssues     = regexp.MustCompile(`^To address .* issues`)
	reBuildRunAudit          = regexp.MustCompile("(?i)^Run `npm (audit|fund)`")
	reBuildFunding           = regexp.MustCompile(`packages are looking for funding`)
)

// buildOutputFilter keeps a build log's errors, its warnings, and its summary,
// and counts the progress lines it drops.
func buildOutputFilter(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return input
	}

	errors := make([]string, 0, 8)
	warnings := make([]string, 0, 8)
	deprecations := make([]string, 0, 8)
	summary := ""
	compiling, downloading := 0, 0
	inCargoError := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inCargoError {
			if trimmed == "" {
				inCargoError = false
				continue
			}
			if reBuildCargoContinuation.MatchString(line) {
				errors = append(errors, line)
				continue
			}
			inCargoError = false
		}
		if trimmed == "" {
			continue
		}

		switch {
		case reBuildNpmError.MatchString(trimmed) || reBuildYarnError.MatchString(trimmed):
			errors = append(errors, line)
		case reBuildDeprecation.MatchString(trimmed):
			deprecations = append(deprecations, line)
		case reBuildNpmWarn.MatchString(trimmed) || reBuildYarnWarn.MatchString(trimmed):
			warnings = append(warnings, line)
		case reBuildErrorHeading.MatchString(trimmed) || strings.HasPrefix(trimmed, "error -->"):
			errors = append(errors, line)
			inCargoError = true
		case reBuildWarnHeading.MatchString(trimmed) || strings.HasPrefix(trimmed, "warning -->"):
			warnings = append(warnings, line)
			inCargoError = true
		case reBuildMavenError.MatchString(trimmed):
			errors = append(errors, line)
		case reBuildBracketError.MatchString(trimmed) || reBuildFailed.MatchString(trimmed):
			errors = append(errors, line)
		case reBuildBracketWarn.MatchString(trimmed):
			warnings = append(warnings, line)
		case reBuildCompiling.MatchString(trimmed):
			compiling++
		case reBuildDownloading.MatchString(trimmed) || reBuildFetching.MatchString(trimmed):
			downloading++
		case isBuildSummary(trimmed):
			if summary == "" {
				summary = line
			} else {
				summary += "\n" + line
			}
		}
	}

	out := &strings.Builder{}
	for _, deprecation := range firstN(deprecations, buildOutputDeprecationKeep) {
		fmt.Fprintf(out, "%s\n", deprecation)
	}
	if len(deprecations) > buildOutputDeprecationKeep {
		fmt.Fprintf(out, "... +%d more deprecated packages\n", len(deprecations)-buildOutputDeprecationKeep)
	}
	if compiling > 0 {
		fmt.Fprintf(out, "Compiled %d packages\n", compiling)
	}
	if downloading > 0 {
		fmt.Fprintf(out, "Downloaded %d packages\n", downloading)
	}
	for _, err := range errors {
		fmt.Fprintf(out, "%s\n", err)
	}
	for _, warning := range firstN(warnings, buildOutputWarningKeep) {
		fmt.Fprintf(out, "%s\n", warning)
	}
	if len(warnings) > buildOutputWarningKeep {
		fmt.Fprintf(out, "... +%d more warnings\n", len(warnings)-buildOutputWarningKeep)
	}
	if summary != "" {
		fmt.Fprintf(out, "%s\n", summary)
	}
	trimmed := strings.TrimRight(out.String(), "\n")
	if trimmed == "" {
		return input
	}
	return trimmed
}

// isBuildSummary reports whether a line is one of the final summary lines the
// reference keeps verbatim.
func isBuildSummary(trimmed string) bool {
	return reBuildSummary.MatchString(trimmed) ||
		reBuildFinished.MatchString(trimmed) ||
		reBuildSuccess.MatchString(trimmed) ||
		reBuildCounts.MatchString(trimmed) ||
		reBuildInstalled.MatchString(trimmed) ||
		reBuildAddressIssues.MatchString(trimmed) ||
		reBuildRunAudit.MatchString(trimmed) ||
		reBuildFunding.MatchString(trimmed)
}
