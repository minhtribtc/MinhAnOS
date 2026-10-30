// Package storage manages data units (user homes, shared folders) on ZFS and Btrfs volumes
// through one capability-based interface (docs/storage-model-review.md). It runs only inside the
// root storage agent: commands are executed directly with validated arguments, never via a shell.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Kind string

const (
	ZFS   Kind = "zfs"
	Btrfs Kind = "btrfs"
)

// Class groups units on a volume; the name becomes a path segment.
type Class string

const (
	Homes  Class = "homes"
	Shares Class = "shares"
)

type Capabilities struct {
	Quota     bool `json:"quota"`
	Snapshots bool `json:"snapshots"`
}

// Usage is the unit's own data; Quota 0 means no limit. Snapshot space is not included.
type Usage struct {
	Referenced uint64 `json:"referencedBytes"`
	Quota      uint64 `json:"quotaBytes"`
	Available  uint64 `json:"availableBytes"`
}

type Snapshot struct {
	Name    string    `json:"name"`
	Created time.Time `json:"createdAt"`
}

type Volume interface {
	Kind() Kind
	Capabilities() Capabilities
	// Create makes a new unit and returns its mount path; ErrExists if it is already there.
	Create(ctx context.Context, class Class, id string) (string, error)
	Path(class Class, id string) (string, error)
	// SetQuota sets a hard limit on the unit's own data; 0 removes the limit.
	SetQuota(ctx context.Context, class Class, id string, limit uint64) error
	Usage(ctx context.Context, class Class, id string) (Usage, error)
	Snapshot(ctx context.Context, class Class, id, name string) error
	Snapshots(ctx context.Context, class Class, id string) ([]Snapshot, error)
	// Destroy removes a unit and its snapshots. The agent exposes it only for empty or tombstoned units.
	Destroy(ctx context.Context, class Class, id string) error
}

var (
	ErrExists   = errors.New("storage unit already exists")
	ErrNotFound = errors.New("storage unit not found")
	ErrInvalid  = errors.New("invalid storage name")
)

// Unit IDs are immutable storage keys (64-hex user IDs) or share slugs; snapshot names are similar.
var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,127}$`)
	snapPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
)

func validUnit(class Class, id string) error {
	if (class != Homes && class != Shares) || !idPattern.MatchString(id) {
		return ErrInvalid
	}
	return nil
}

// run executes one storage command; output is returned trimmed and errors carry stderr.
func run(ctx context.Context, name string, args ...string) (string, error) {
	var out, errOut bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if e := cmd.Run(); e != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), e, strings.TrimSpace(errOut.String()))
	}
	return strings.TrimSpace(out.String()), nil
}
