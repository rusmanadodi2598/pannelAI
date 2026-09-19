// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_git_status.go
// @for       The git-status filter: a status dump reduced to one branch line
//
//	and per-group file counts.
//
// @uses      fmt, regexp, strings.
// @reason    SPEC-API-002 §5 ports the reference's format_status_output, which
//
//	reads both the long form a human runs and the porcelain form an
//	agent runs. The compressed answer tells the model what changed
//	without spending a line per file.
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
	reStatusBranch    = regexp.MustCompile(`^On branch (\S+)`)
	reStatusPorcelain = regexp.MustCompile(`^[ MADRCU?!][ MADRCU?!] `)
	reStatusLong      = regexp.MustCompile(`^\s*(modified|new file|deleted|renamed|both modified):\s+(.+)$`)
)

// gitStatusFilter compacts a `git status` dump, in either the long form or the
// porcelain form.
func gitStatusFilter(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return "Clean working tree"
	}

	branch := ""
	staged, modified, untracked, conflicts := 0, 0, 0, 0
	stagedFiles := make([]string, 0, 8)
	modifiedFiles := make([]string, 0, 8)
	untrackedFiles := make([]string, 0, 8)

	for _, raw := range lines {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if match := reStatusBranch.FindStringSubmatch(raw); match != nil {
			branch = match[1]
			continue
		}
		if strings.HasPrefix(raw, "##") {
			branch = strings.TrimSpace(strings.TrimPrefix(raw, "##"))
			continue
		}
		if len(raw) >= 3 && reStatusPorcelain.MatchString(raw) {
			file := raw[3:]
			if raw[:2] == "??" {
				untracked++
				untrackedFiles = append(untrackedFiles, file)
				continue
			}
			if strings.ContainsAny(raw[:1], "MADRC") {
				staged++
				stagedFiles = append(stagedFiles, file)
			} else if raw[0] == 'U' {
				conflicts++
			}
			if raw[1] == 'M' || raw[1] == 'D' {
				modified++
				modifiedFiles = append(modifiedFiles, file)
			}
			continue
		}
		if match := reStatusLong.FindStringSubmatch(raw); match != nil {
			kind, path := match[1], strings.TrimSpace(match[2])
			switch kind {
			case "both modified":
				conflicts++
			case "modified", "deleted":
				modified++
				modifiedFiles = append(modifiedFiles, path)
			case "new file", "renamed":
				staged++
				stagedFiles = append(stagedFiles, path)
			}
		}
	}

	out := &strings.Builder{}
	if branch != "" {
		fmt.Fprintf(out, "* %s\n", branch)
	}
	writeStatusGroup(out, "+ Staged", staged, stagedFiles, statusMaxFiles)
	writeStatusGroup(out, "~ Modified", modified, modifiedFiles, statusMaxFiles)
	writeStatusGroup(out, "? Untracked", untracked, untrackedFiles, statusMaxUntracked)
	if conflicts > 0 {
		fmt.Fprintf(out, "conflicts: %d files\n", conflicts)
	}
	if staged == 0 && modified == 0 && untracked == 0 && conflicts == 0 {
		out.WriteString("clean: nothing to commit\n")
	}
	return strings.TrimRight(out.String(), "\n")
}

// writeStatusGroup renders one status group and its capped file list.
func writeStatusGroup(out *strings.Builder, label string, count int, files []string, cap int) {
	if count == 0 {
		return
	}
	fmt.Fprintf(out, "%s: %d files\n", label, count)
	for _, file := range firstN(files, cap) {
		fmt.Fprintf(out, "   %s\n", file)
	}
	if len(files) > cap {
		fmt.Fprintf(out, "   ... +%d more\n", len(files)-cap)
	}
}
