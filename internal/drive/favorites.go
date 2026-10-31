package drive

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/storagecore"
)

// MaxFavorites bounds the favorites of one user (files SQL 007).
const MaxFavorites = 500

// Favorite is one item pinned in the Files sidebar; Dir tells Files to open it as a folder.
type Favorite struct {
	Root    string    `json:"root"`
	Path    string    `json:"path"`
	Dir     bool      `json:"dir"`
	Created time.Time `json:"created"`
}

// FavoriteStore keeps favorites; nil disables them (tests, standalone without a database).
type FavoriteStore interface {
	List(ctx context.Context, user string) ([]Favorite, error)
	// Add returns false when the user already has MaxFavorites (an existing favorite is still updated). Two adds racing
	// at the limit may pass it by one; the bound is a guard, not an exact count.
	Add(ctx context.Context, user string, f Favorite) (bool, error)
	Remove(ctx context.Context, user, root, path string) error
	// Retarget follows a rename or move (to empty: removal); see SharingStore.Retarget.
	Retarget(ctx context.Context, owner, root, from, to string) error
}

type SQLFavorites struct{ DB *pgxpool.Pool }

func (s SQLFavorites) List(ctx context.Context, user string) ([]Favorite, error) {
	rows, err := s.DB.Query(ctx, "SELECT root,path,dir,created_at FROM files_favorites WHERE user_id=$1 ORDER BY created_at, path", user)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Favorite, error) {
		var f Favorite
		return f, r.Scan(&f.Root, &f.Path, &f.Dir, &f.Created)
	})
}

func (s SQLFavorites) Add(ctx context.Context, user string, f Favorite) (bool, error) {
	tag, err := s.DB.Exec(ctx, `INSERT INTO files_favorites(user_id,root,path,dir) SELECT $1,$2,$3,$4
		WHERE (SELECT count(*) FROM files_favorites WHERE user_id=$1) < $5 OR EXISTS (SELECT 1 FROM files_favorites WHERE user_id=$1 AND root=$2 AND path=$3)
		ON CONFLICT (user_id,root,path) DO UPDATE SET dir=EXCLUDED.dir`, user, f.Root, f.Path, f.Dir, MaxFavorites)
	return err == nil && tag.RowsAffected() > 0, err
}

func (s SQLFavorites) Retarget(ctx context.Context, owner, root, from, to string) error {
	return SQLSharing{DB: s.DB}.Retarget(ctx, owner, root, from, to)
}

func (s SQLFavorites) Remove(ctx context.Context, user, root, path string) error {
	_, err := s.DB.Exec(ctx, "DELETE FROM files_favorites WHERE user_id=$1 AND root=$2 AND path=$3", user, root, path)
	return err
}

// serveFavorites: GET /api/favorites lists the favorites in roots the user can still open; POST /api/favorites
// adds {root,path,dir}; POST /api/favorites/remove removes {root,path}. Paths are not checked against the disk:
// a favorite whose item is gone simply fails to open, like a moved bookmark.
func (s *Server) serveFavorites(w http.ResponseWriter, r *http.Request, u User) {
	if s.Favorites == nil {
		fail(w, 503, "Mục yêu thích chưa được bật.", "SERVICE_UNAVAILABLE")
		return
	}
	if !validID(u.ID) {
		fail(w, 403, "Định danh người dùng không hợp lệ.", "ACCESS_DENIED")
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/api/favorites":
		list, err := s.Favorites.List(r.Context(), u.ID)
		if err != nil {
			logError(r.Context(), "files.favorites.list_failed", err)
			fail(w, 503, "Không đọc được mục yêu thích.", "SERVICE_UNAVAILABLE")
			return
		}
		out := make([]Favorite, 0, len(list))
		// Favorites of items that are gone are dropped here, whatever removed them.
		engines := map[string]*Engine{}
		visible := map[string]bool{}
		defer func() {
			for _, e := range engines {
				if e != nil {
					e.Root.Close()
				}
			}
		}()
		for _, f := range list {
			ok, seen := visible[f.Root]
			if !seen {
				e, _, found, err := s.favoriteEngine(r.Context(), u, f.Root)
				if err != nil {
					fail(w, 503, "Không đọc được cấu hình lưu trữ.", "SERVICE_UNAVAILABLE")
					return
				}
				engines[f.Root], visible[f.Root], ok = e, found, found
			}
			if !ok {
				continue
			}
			if favoriteGone(engines[f.Root], f.Path) {
				if err := s.Favorites.Remove(r.Context(), u.ID, f.Root, f.Path); err != nil {
					logError(r.Context(), "files.favorites.remove_failed", err)
				}
				continue
			}
			out = append(out, f)
		}
		jsonResponse(w, 200, map[string]any{"items": out, "limit": MaxFavorites})
	case r.Method == "POST" && (r.URL.Path == "/api/favorites" || r.URL.Path == "/api/favorites/remove"):
		var body struct {
			Root string `json:"root"`
			Path string `json:"path"`
			Dir  bool   `json:"dir"`
		}
		if err := decodeBody(r, &body); err != nil {
			handleFileError(w, err)
			return
		}
		p, err := clean(body.Path)
		if err != nil || p == "." {
			fail(w, 400, "Đường dẫn không hợp lệ.", "FILES_FAVORITE_INVALID")
			return
		}
		if r.URL.Path == "/api/favorites/remove" {
			if err = s.Favorites.Remove(r.Context(), u.ID, body.Root, p); err != nil {
				logError(r.Context(), "files.favorites.remove_failed", err)
				fail(w, 503, "Không lưu được mục yêu thích.", "SERVICE_UNAVAILABLE")
				return
			}
			jsonResponse(w, 200, map[string]bool{"ok": true})
			return
		}
		if ok, err := s.favoriteRoot(r.Context(), u, body.Root); err != nil {
			fail(w, 503, "Không đọc được cấu hình lưu trữ.", "SERVICE_UNAVAILABLE")
			return
		} else if !ok {
			fail(w, 400, "Không gian lưu trữ không hợp lệ.", "FILES_ROOT_INVALID")
			return
		}
		added, err := s.Favorites.Add(r.Context(), u.ID, Favorite{Root: body.Root, Path: p, Dir: body.Dir})
		if err != nil {
			logError(r.Context(), "files.favorites.add_failed", err)
			fail(w, 503, "Không lưu được mục yêu thích.", "SERVICE_UNAVAILABLE")
			return
		}
		if !added {
			fail(w, 409, "Đã đạt giới hạn mục yêu thích.", "FILES_FAVORITES_LIMIT")
			return
		}
		jsonResponse(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 405, "Phương thức không được hỗ trợ.", "METHOD_NOT_ALLOWED")
	}
}

// favoriteRoot reports whether the user may open root now: the home, a Shared Folder they can read, an item shared
// with them, or the host system root for operators.
func (s *Server) favoriteRoot(ctx context.Context, u User, root string) (bool, error) {
	e, done, ok, err := s.favoriteEngine(ctx, u, root)
	if e != nil {
		done()
	}
	return ok, err
}

// favoriteEngine opens root for an existence check (nil engine for the system root, which is not checked).
func (s *Server) favoriteEngine(ctx context.Context, u User, root string) (*Engine, func(), bool, error) {
	none := func() {}
	open := func(dir string, err error) (*Engine, func(), bool, error) {
		if errors.Is(err, storagecore.ErrNoHomes) || errors.Is(err, fs.ErrNotExist) {
			return nil, none, false, nil
		}
		if err != nil {
			return nil, none, false, err
		}
		r, err := os.OpenRoot(dir)
		if err != nil {
			return nil, none, false, nil
		}
		return &Engine{Root: r}, func() { r.Close() }, true, nil
	}
	switch {
	case root == "personal":
		// The home always exists for the user, even before its first write creates the folder.
		e, done, _, err := open(s.homeDir(ctx, u.ID, false))
		return e, done, err == nil, err
	case root == "system":
		ok, err := s.hostAllowed(ctx, u)
		return nil, none, ok, err
	case strings.HasPrefix(root, "share:"):
		share, ok, err := s.userShare(ctx, u.ID, strings.TrimPrefix(root, "share:"))
		if err != nil {
			logError(ctx, "files.shares.lookup_failed", err)
		}
		if !ok || err != nil {
			return nil, none, false, err
		}
		return open(s.unitPath(ctx, share.Volume, "shares", share.Key, false))
	case strings.HasPrefix(root, "shared:") && s.Sharing != nil:
		g, ok, err := s.Sharing.ReceivedGrant(ctx, strings.TrimPrefix(root, "shared:"), u.ID)
		if !ok || err != nil {
			return nil, none, false, err
		}
		dir, err := s.homeDir(ctx, g.OwnerID, false)
		if err != nil {
			return nil, none, false, nil
		}
		e, r, err := sharedEngine(dir, g.Path, g.Dir)
		if err != nil {
			return nil, none, false, nil
		}
		return e, func() { r.Close() }, true, nil
	}
	return nil, none, false, nil
}

// favoriteGone reports a favorite whose item no longer exists (deleted, moved or renamed outside Files).
func favoriteGone(e *Engine, p string) bool {
	if e == nil {
		return false
	}
	q, err := e.logical(p)
	if err != nil {
		return true
	}
	_, err = e.Root.Lstat(e.disk(q))
	return errors.Is(err, fs.ErrNotExist)
}
