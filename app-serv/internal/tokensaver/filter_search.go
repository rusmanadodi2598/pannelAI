// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_search.go
// @for       The three search-shaped filters: grep, find, and search-list.
// @uses      fmt, regexp, sort, strings.
// @reason    SPEC-API-002 §5 ports the reference's grep_wrapper, find_wrapper,
//
//	and search-list filters. All three answer "where is this?", and all
//	three are regrouped by file or directory so the model reads the
//	shape of the result rather than every match.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     util
// @stability experimental
// @since     2026-09-19
package tokensaver

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// reSearchListHeader matches the Cursor Glob tool's result header.
var reSearchListHeader = regexp.MustCompile(`^Result of search in '[^']*' \(total \d+ files?\):`)

// grepFilter regroups a "file:line:content" match list by file.
func grepFilter(input string) string {
	byFile := map[string][][2]string{}
	total := 0
	for _, line := range strings.Split(input, "\n") {
		first := strings.Index(line, ":")
		if first < 0 {
			continue
		}
		second := strings.Index(line[first+1:], ":")
		if second < 0 {
			continue
		}
		file := line[:first]
		number := line[first+1 : first+1+second]
		if !reGrepLineNo.MatchString(number) {
			continue
		}
		total++
		byFile[file] = append(byFile[file], [2]string{number, line[first+1+second+1:]})
	}
	if total == 0 {
		return input
	}

	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)

	out := &strings.Builder{}
	fmt.Fprintf(out, "%d matches in %dF:\n\n", total, len(files))
	for _, file := range files {
		matches := byFile[file]
		fmt.Fprintf(out, "[file] %s (%d):\n", file, len(matches))
		shown := matches
		if len(shown) > grepPerFileMax {
			shown = shown[:grepPerFileMax]
		}
		for _, match := range shown {
			fmt.Fprintf(out, "  %4s: %s\n", match[0], strings.TrimSpace(match[1]))
		}
		if len(matches) > grepPerFileMax {
			fmt.Fprintf(out, "  +%d\n", len(matches)-grepPerFileMax)
		}
		out.WriteString("\n")
	}
	return out.String()
}

// findFilter regroups a path list by parent directory.
func findFilter(input string) string {
	lines := make([]string, 0, 16)
	for _, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return input
	}

	byDir := map[string][]string{}
	for _, path := range lines {
		dir, base := splitPath(path)
		byDir[dir] = append(byDir[dir], base)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	out := &strings.Builder{}
	fmt.Fprintf(out, "%d files in %d dirs:\n\n", len(lines), len(dirs))
	for _, dir := range firstN(dirs, findTotalDirMax) {
		files := byDir[dir]
		fmt.Fprintf(out, "%s/  (%d)\n", strings.ReplaceAll(dir, `\`, "/"), len(files))
		for _, file := range firstN(files, findPerDirMax) {
			fmt.Fprintf(out, "  %s\n", file)
		}
		if len(files) > findPerDirMax {
			fmt.Fprintf(out, "  +%d\n", len(files)-findPerDirMax)
		}
	}
	if len(dirs) > findTotalDirMax {
		fmt.Fprintf(out, "\n+%d more dirs\n", len(dirs)-findTotalDirMax)
	}
	return out.String()
}

// searchListFilter compacts the Cursor Glob result list: a header line, then
// "- path" entries regrouped by directory.
func searchListFilter(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return input
	}
	header, rest := lines[0], lines[1:]

	paths := make([]string, 0, len(rest))
	for _, raw := range rest {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "- ") {
			paths = append(paths, strings.TrimPrefix(trimmed, "- "))
		}
	}
	if len(paths) == 0 {
		return input
	}

	byDir := map[string][]string{}
	for _, path := range paths {
		slash := strings.LastIndex(path, "/")
		dir, name := ".", path
		if slash >= 0 {
			dir = path[:slash]
			if dir == "" {
				dir = "/"
			}
			name = path[slash+1:]
		}
		byDir[dir] = append(byDir[dir], name)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	out := &strings.Builder{}
	fmt.Fprintf(out, "%s\n%d files in %d dirs:\n\n", header, len(paths), len(dirs))
	for _, dir := range firstN(dirs, searchListTotalDirMax) {
		names := byDir[dir]
		fmt.Fprintf(out, "%s/ (%d):\n", dir, len(names))
		for _, name := range firstN(names, searchListPerDirMax) {
			fmt.Fprintf(out, "  %s\n", name)
		}
		if len(names) > searchListPerDirMax {
			fmt.Fprintf(out, "  +%d\n", len(names)-searchListPerDirMax)
		}
		out.WriteString("\n")
	}
	if len(dirs) > searchListTotalDirMax {
		fmt.Fprintf(out, "+%d more dirs\n", len(dirs)-searchListTotalDirMax)
	}
	return strings.TrimRight(out.String(), "\n")
}

// splitPath splits a path into its parent directory and its base name, treating
// both separators and reporting "." for a bare name.
func splitPath(path string) (string, string) {
	last := strings.LastIndexAny(path, `/\`)
	if last < 0 {
		return ".", path
	}
	dir := path[:last]
	if dir == "" {
		dir = "/"
	}
	return dir, path[last+1:]
}
