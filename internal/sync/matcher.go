package sync

import (
	"path/filepath"
	"strings"
)

// Matcher checks file paths against ignore patterns.
// Patterns are matched against every segment of a path
// so `.git/` catches `.git` at any depth without needing `**`
type Matcher struct {
	rules []rule
}

type rule struct {
	raw     string
	glob    string
	dirOnly bool
}

// NewMatcher compiles ignore patterns from local config.
// A trailing slash means "directories only".
// A `**/` prefix is stripped for compatibility with gitignore syntax.
func NewMatcher(patterns []string) *Matcher {
	rules := make([]rule, 0, len(patterns))
	for _, p := range patterns {
		r := rule{raw: p}

		if strings.HasSuffix(p, "/") {
			r.dirOnly = true
			p = p[:len(p)-1]
		}

		strings.TrimPrefix(p, "**/")

		r.glob = p
		rules = append(rules, r)
	}
	return &Matcher{rules: rules}
}

// Match reports whether path should be ignored.
// isDir indicates whether path corresponds to a directory.
func (m *Matcher) Match(path string, isDir bool) bool {
	segments := strings.Split(path, string(filepath.Separator))

	for _, r := range m.rules {
		if r.dirOnly && !isDir {
			continue
		}
		for _, seg := range segments {
			matched, _ := filepath.Match(r.glob, seg)
			if matched {
				return true
			}
		}
	}

	return false
}
