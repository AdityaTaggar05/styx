package store

import (
	"context"
	"time"
)

// FileMeta describes a file on the remote store.
type FileMeta struct {
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	IsDir    bool      `json:"is_dir"`
	RemoteID string    `json:"remote_id"`
}

// ChangeType classifies a remote change event.
type ChangeType int

const (
	ChangeCreated  ChangeType = iota
	ChangeModified
	ChangeDeleted
)

// Change represents a single remote change event.
type Change struct {
	Type     ChangeType `json:"type"`
	Path     string     `json:"path"`
	FileMeta *FileMeta  `json:"file_meta,omitempty"`
}

// DataStore defines the contract every cloud storage backend must satisfy.
type DataStore interface {
	Name() string

	// Auth
	AuthURL() string
	Authenticate(ctx context.Context, code string) error
	IsAuthenticated(ctx context.Context) (bool, error)
	Logout(ctx context.Context) error

	// File CRUD
	Pull(ctx context.Context, remotePath string, localPath string) error
	Push(ctx context.Context, localPath string, remotePath string) (*FileMeta, error)
	Delete(ctx context.Context, remotePath string) error

	// Metadata
	Hash(ctx context.Context, remotePath string) (string, error)
	List(ctx context.Context, remotePath string) ([]FileMeta, error)
	ListRecursive(ctx context.Context, remotePath string) ([]FileMeta, error)

	// Incremental change detection — cursor is an opaque token from the last call.
	ChangesSince(ctx context.Context, cursor string) ([]Change, string, error)
}
