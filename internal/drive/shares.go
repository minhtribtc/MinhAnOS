package drive

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"

	"minhanos.local/platform/internal/storagecore"
)

// ShareRegistry is Files' view of the storage registry (storagecore.Store in production): where homes live,
// which Shared Folders a user may open, and the share quota ledger. Nil keeps the legacy layout only
// (homes in DataDir/users, no Shared Folders).
type ShareRegistry interface {
	HomesShare(ctx context.Context) (storagecore.HomesShare, error)
	UserShares(ctx context.Context, storageID string) ([]storagecore.UserShare, error)
	ReserveShare(ctx context.Context, share string, folders []string, bytes int64) (storagecore.Reservation, error)
	// Quota folders (Q3) and background measurement of folders and shares.
	ShareFolders(ctx context.Context, shareID string) ([]storagecore.FolderScope, error)
	MoveFolders(ctx context.Context, shareID, from, to string) error
	AdjustFolders(ctx context.Context, folders []string, delta int64) error
	PendingFolderMeasures(ctx context.Context, limit int) ([]storagecore.PendingFolder, error)
	SetFolderMeasured(ctx context.Context, id string, bytes int64) error
	PendingShareMeasures(ctx context.Context, limit int) ([]storagecore.PendingShare, error)
	SetShareMeasured(ctx context.Context, id string, bytes int64) error
	RequestShareMeasure(ctx context.Context, id string) error
	QuotaFolderUsage(ctx context.Context, shareID string) ([]storagecore.QuotaFolder, error)
}

// UnitAgent resolves ZFS/Btrfs units to host paths (storagecore.AgentClient in production). Files reaches
// those paths through the /srv/minhanos bind mount; it never creates datasets itself.
type UnitAgent interface {
	Unit(ctx context.Context, volume, class, id string) (storagecore.AgentUnit, error)
	CreateUnit(ctx context.Context, volume, class, id string) (storagecore.AgentUnit, error)
}

// unitPath returns the unit's path, creating it when asked (homes on first use). Paths are cached: a unit
// never moves while it exists.
func (s *Server) unitPath(ctx context.Context, volume, class, id string, create bool) (string, error) {
	key := volume + "/" + class + "/" + id
	if v, ok := s.units.Load(key); ok {
		return v.(string), nil
	}
	if s.Units == nil {
		return "", errors.New("storage agent not configured")
	}
	unit, e := s.Units.Unit(ctx, volume, class, id)
	var agent storagecore.AgentError
	if errors.As(e, &agent) && agent.Status == 404 {
		if !create {
			return "", fs.ErrNotExist
		}
		unit, e = s.Units.CreateUnit(ctx, volume, class, id)
	}
	if e != nil {
		return "", e
	}
	s.units.Store(key, unit.Path)
	return unit.Path, nil
}

// homeDir is the user's home: the legacy Files data directory, or the unit homes/<user> on the volume of
// the share hosting homes. storagecore.ErrNoHomes means the first System Admin has not created that share.
func (s *Server) homeDir(ctx context.Context, user string, create bool) (string, error) {
	if s.Shares == nil {
		return filepath.Join(s.DataDir, "users", user), nil
	}
	h, e := s.Shares.HomesShare(ctx)
	if e != nil {
		return "", e
	}
	if h.VolumeKind == "directory" {
		return filepath.Join(s.DataDir, "users", user), nil
	}
	return s.unitPath(ctx, h.Volume, "homes", user, create)
}

// userShare finds a Shared Folder the user may open; deny and no entry both read as not found.
func (s *Server) userShare(ctx context.Context, user, key string) (storagecore.UserShare, bool, error) {
	if s.Shares == nil {
		return storagecore.UserShare{}, false, nil
	}
	shares, e := s.Shares.UserShares(ctx, user)
	if e != nil {
		return storagecore.UserShare{}, false, e
	}
	for _, share := range shares {
		if share.Key == key {
			return share, true, nil
		}
	}
	return storagecore.UserShare{}, false, nil
}
