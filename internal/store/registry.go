package store

import (
	"fmt"
	"sort"
	"sync"

	styxErrors "github.com/AdityaTaggar05/styx/internal/errors"
)

// Constructor is a factory function that creates a DataStore from a config map.
type Constructor func(cfg map[string]string) (DataStore, error)

var (
	mu       sync.RWMutex
	backends = make(map[string]Constructor)
)

// Register adds a backend constructor to the registry. Call from init().
func Register(name string, ctor Constructor) {
	mu.Lock()
	defer mu.Unlock()
	backends[name] = ctor
}

// Get creates a DataStore for the named backend using the provided config map.
func Get(name string, cfg map[string]string) (DataStore, error) {
	mu.RLock()
	ctor, ok := backends[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", styxErrors.ErrStoreUnknown, name)
	}
	return ctor(cfg)
}

// List returns the names of all registered backends.
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
