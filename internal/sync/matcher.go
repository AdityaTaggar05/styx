package sync

import (
	"path"
	"path/filepath"
	"strings"
)

// Matcher checks file paths against ignore patterns.
//
// A pattern without a slash is matched against every path segment, so
// `node_modules` (or `node_modules/`) ignores that directory at any depth.
// A pattern containing a slash is matched against the full relative path, so
// `.trash/*` or `docs/notes.md` behave as expected. A trailing slash means
// "directories only", and a leading `**/` is stripped.
type Matcher struct {
	rules []rule
}

type rule struct {
	raw      string
	glob     string
	dirOnly  bool
	hasSlash bool
}

// NewMatcher compiles ignore patterns from local config.
func NewMatcher(patterns []string) *Matcher {
	rules := make([]rule, 0, len(patterns))
	for _, p := range patterns {
		r := rule{raw: p}

		if strings.HasSuffix(p, "/") {
			r.dirOnly = true
			p = strings.TrimSuffix(p, "/")
		}

		p = strings.TrimPrefix(p, "**/")

		r.glob = p
		r.hasSlash = strings.Contains(p, "/")
		rules = append(rules, r)
	}
	return &Matcher{rules: rules}
}

// Match reports whether relPath should be ignored.
// isDir indicates whether relPath corresponds to a directory.
//
// A matching directory also ignores everything beneath it: intermediate path
// segments are always directories, and a directory-only pattern matches them.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	slashPath := filepath.ToSlash(relPath)
	segments := strings.Split(relPath, string(filepath.Separator))

	for _, r := range m.rules {
		// Patterns with a slash are anchored to the root and match the whole
		// relative path.
		if r.hasSlash {
			if r.dirOnly {
				if isDir {
					if matched, _ := path.Match(r.glob, slashPath); matched {
						return true
					}
				}
				if strings.HasPrefix(slashPath, strings.TrimSuffix(r.glob, "/")+"/") {
					return true
				}
				continue
			}
			if matched, _ := path.Match(r.glob, slashPath); matched {
				return true
			}
			continue
		}

		// Patterns without a slash match any path segment. For a directory-only
		// pattern, a matching final segment counts only if it is a directory;
		// intermediate segments are always directories.
		for i, seg := range segments {
			if r.dirOnly && i == len(segments)-1 && !isDir {
				continue
			}
			if matched, _ := filepath.Match(r.glob, seg); matched {
				return true
			}
		}
	}

	return false
}
