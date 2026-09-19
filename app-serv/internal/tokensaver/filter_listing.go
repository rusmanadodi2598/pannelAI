// Package tokensaver implements the native token savers of SPEC-API-002: the
// RTK tool-result compressor, the ponytail system-prompt injector, and the
// headroom external-compression client.
//
// @file      internal/tokensaver/filter_listing.go
// @for       The two listing filters: ls and tree.
// @uses      fmt, regexp, sort, strconv, strings.
// @reason    SPEC-API-002 §5 ports the reference's compact_ls and
//
//	filter_tree_output. A directory listing is read for its shape, so
//	the ls filter drops the noise directories and summarizes the
//	extensions, and the tree filter drops the counts line.
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
	"strconv"
	"strings"
)

// reLsDate matches the month-day-time column of an `ls -la` row.
var reLsDate = regexp.MustCompile(`\s+(Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+(\d{4}|\d{2}:\d{2})\s+`)

// lsFilter compacts an `ls -la` listing: directories first, then files with
// human sizes, then a summary of the extensions.
func lsFilter(input string) string {
	dirs := make([]string, 0, 16)
	files := make([][2]string, 0, 16)
	byExt := map[string]int{}
	// extOrder keeps the first-appearance order, so two extensions with the
	// same count always render in the same order.
	extOrder := make([]string, 0, 8)

	for _, line := range strings.Split(input, "\n") {
		if strings.HasPrefix(line, "total ") || line == "" {
			continue
		}
		entry, ok := parseLsLine(line)
		if !ok || entry.name == "." || entry.name == ".." || lsNoiseDirs[entry.name] {
			continue
		}
		switch entry.fileType {
		case "d":
			dirs = append(dirs, entry.name)
		case "-", "l":
			ext := "no ext"
			if dot := strings.LastIndex(entry.name, "."); dot > 0 {
				ext = entry.name[dot:]
			}
			if byExt[ext] == 0 {
				extOrder = append(extOrder, ext)
			}
			byExt[ext]++
			files = append(files, [2]string{entry.name, humanSize(entry.size)})
		}
	}
	if len(dirs) == 0 && len(files) == 0 {
		return input
	}

	out := &strings.Builder{}
	for _, dir := range dirs {
		fmt.Fprintf(out, "%s/\n", dir)
	}
	for _, file := range files {
		fmt.Fprintf(out, "%s  %s\n", file[0], file[1])
	}
	fmt.Fprintf(out, "\nSummary: %d files, %d dirs", len(files), len(dirs))
	writeExtSummary(out, extOrder, byExt)
	return out.String()
}

// lsEntry is one parsed `ls -la` row.
type lsEntry struct {
	fileType string
	size     int
	name     string
}

// parseLsLine reads one `ls -la` row, reporting false for a line that is not
// one: the header, a blank, or anything without the date column.
func parseLsLine(line string) (lsEntry, bool) {
	loc := reLsDate.FindStringIndex(line)
	if loc == nil {
		return lsEntry{}, false
	}
	name := line[loc[1]:]
	parts := strings.Fields(line[:loc[0]])
	if len(parts) < 4 {
		return lsEntry{}, false
	}
	size := 0
	for i := len(parts) - 1; i >= 0; i-- {
		if parsed, err := strconv.Atoi(parts[i]); err == nil && strconv.Itoa(parsed) == parts[i] {
			size = parsed
			break
		}
	}
	return lsEntry{fileType: parts[0][:1], size: size, name: name}, true
}

// humanSize renders a byte count the way the reference does.
func humanSize(bytes int) string {
	switch {
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(bytes)/(1<<20))
	case bytes >= 1024:
		return fmt.Sprintf("%.1fK", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// writeExtSummary appends the extension summary, naming the top few. Ties keep
// the order the extensions first appeared in.
func writeExtSummary(out *strings.Builder, order []string, byExt map[string]int) {
	if len(order) == 0 {
		return
	}
	exts := append([]string{}, order...)
	sort.SliceStable(exts, func(i, j int) bool { return byExt[exts[i]] > byExt[exts[j]] })

	parts := make([]string, 0, lsExtSummaryTop)
	for _, ext := range firstN(exts, lsExtSummaryTop) {
		parts = append(parts, fmt.Sprintf("%d %s", byExt[ext], ext))
	}
	fmt.Fprintf(out, " (%s", strings.Join(parts, ", "))
	if len(exts) > lsExtSummaryTop {
		fmt.Fprintf(out, ", +%d more", len(exts)-lsExtSummaryTop)
	}
	out.WriteString(")")
}

// treeFilter drops a tree listing's summary line and its blank edges, and caps
// an oversized listing.
func treeFilter(input string) string {
	lines := strings.Split(input, "\n")
	if len(lines) == 0 {
		return input
	}
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "director") && strings.Contains(line, "file") {
			continue
		}
		if strings.TrimSpace(line) == "" && len(filtered) == 0 {
			continue
		}
		filtered = append(filtered, line)
	}
	for len(filtered) > 0 && strings.TrimSpace(filtered[len(filtered)-1]) == "" {
		filtered = filtered[:len(filtered)-1]
	}
	if len(filtered) > treeMaxLines {
		cut := len(filtered) - treeMaxLines
		return strings.Join(filtered[:treeMaxLines], "\n") + fmt.Sprintf("\n... +%d more lines", cut)
	}
	return strings.Join(filtered, "\n")
}
