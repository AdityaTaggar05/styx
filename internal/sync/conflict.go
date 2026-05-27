package sync

import (
	"fmt"
	"os"
	"path/filepath"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// SaveConflict renames the local file to preserve local changes before the
// remote version overwrites. The suffix is inserted before the file extension
// (e.g. file.md → file.conflict.md, Makefile → Makefile.conflict).
func SaveConflict(localPath, suffix string) (string, error) {
	dir := filepath.Dir(localPath)
	base := filepath.Base(localPath)

	var conflictName string
	ext := filepath.Ext(base)
	if ext == "" {
		conflictName = base + suffix
	} else {
		conflictName = base[:len(base)-len(ext)] + suffix + ext
	}

	conflictPath := filepath.Join(dir, conflictName)

	if err := os.Rename(localPath, conflictPath); err != nil {
		return "", fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return conflictPath, nil
}
