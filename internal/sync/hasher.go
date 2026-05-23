package sync

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// HashFile computes the SHA-256 hash of a file at path.
// Returns the hash as "sha256:<hex>" for self-describing storage.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", styxErrors.ErrFileNotFound, path)
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("%w: %s", styxErrors.ErrFilePermission, path)
		}
		return "", fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("%w: %v", styxErrors.ErrFileIO, err)
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
