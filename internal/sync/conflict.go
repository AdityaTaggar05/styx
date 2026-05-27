package sync

import (
	"fmt"
	"os"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// SaveConflict renames the local file to <path><suffix> to preserve local
// changes before the remote version overwrites. Returns the conflict path.
func SaveConflict(localPath, suffix string) (string, error) {
	conflictPath := localPath + suffix

	if err := os.Rename(localPath, conflictPath); err != nil {
		return "", fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return conflictPath, nil
}
