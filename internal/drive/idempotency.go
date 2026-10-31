package drive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// Every write to /api/files/* needs an Idempotency-Key (app standard row 15). A repeated key with the same
// request replays the stored response for IdempotencyTTL; the same key with another request is refused.
const IdempotencyTTL = 24 * time.Hour

var ErrIdempotencyConflict = errors.New("idempotency key reused for another request")

// IdempotencyStore keeps successful write responses; SQLIdentity implements it (files SQL 002). Without a
// store the key is still required but nothing is replayed.
type IdempotencyStore interface {
	Replay(ctx context.Context, user, keyHash, requestHash string) (status int, body []byte, found bool, err error)
	Remember(ctx context.Context, user, keyHash, requestHash string, status int, body []byte) error
}

func (s *SQLIdentity) Replay(ctx context.Context, user, keyHash, requestHash string) (int, []byte, bool, error) {
	var stored string
	var status int
	var body []byte
	err := s.db.QueryRow(ctx, "SELECT request_hash,status,response FROM files_idempotency WHERE user_id=$1 AND key_hash=$2 AND expires_at>now()", user, keyHash).Scan(&stored, &status, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, err
	}
	if stored != requestHash {
		return 0, nil, false, ErrIdempotencyConflict
	}
	return status, body, true, nil
}

func (s *SQLIdentity) Remember(ctx context.Context, user, keyHash, requestHash string, status int, body []byte) error {
	_, err := s.db.Exec(ctx, `INSERT INTO files_idempotency(user_id,key_hash,request_hash,status,response,expires_at) VALUES($1,$2,$3,$4,$5,now()+make_interval(secs=>$6))
		ON CONFLICT(user_id,key_hash) DO UPDATE SET request_hash=EXCLUDED.request_hash,status=EXCLUDED.status,response=EXCLUDED.response,expires_at=EXCLUDED.expires_at WHERE files_idempotency.expires_at<=now()`,
		user, keyHash, requestHash, status, body, IdempotencyTTL.Seconds())
	return err
}

// replayRecorder keeps a copy of a write response so it can be replayed for the same key.
type replayRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *replayRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
func (r *replayRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// idempotent checks the key of a write. done means the response was written (missing key, conflict or a
// replay). Otherwise the caller writes through rec and calls finish when the write is over.
func (s *Server) idempotent(w http.ResponseWriter, r *http.Request, u User) (rec *replayRecorder, finish func(), done bool) {
	key := r.Header.Get("Idempotency-Key")
	if len(key) < 16 || len(key) > 128 {
		fail(w, 400, "Thiếu Idempotency-Key.", "IDEMPOTENCY_KEY_REQUIRED")
		return nil, nil, true
	}
	// The request is the operation, its query and body; an upload is identified by its declared size
	// instead of its (possibly large) body.
	h := sha256.New()
	io.WriteString(h, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+"\x00")
	if r.URL.Path == "/api/files/upload" {
		io.WriteString(h, r.Header.Get("Content-Length"))
	} else {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			fail(w, 413, "Yêu cầu quá lớn.", "PAYLOAD_TOO_LARGE")
			return nil, nil, true
		}
		h.Write(raw)
		r.Body = io.NopCloser(bytes.NewReader(raw))
	}
	keySum := sha256.Sum256([]byte(key))
	keyHash, requestHash := hex.EncodeToString(keySum[:]), hex.EncodeToString(h.Sum(nil))
	store, ok := s.Identity.(IdempotencyStore)
	rec = &replayRecorder{ResponseWriter: w, status: 200}
	if !ok {
		return rec, func() {}, false
	}
	status, body, found, err := store.Replay(r.Context(), u.ID, keyHash, requestHash)
	switch {
	case errors.Is(err, ErrIdempotencyConflict):
		fail(w, 409, "Idempotency-Key đã dùng cho yêu cầu khác.", "IDEMPOTENCY_CONFLICT")
		return nil, nil, true
	case err != nil:
		logError(r.Context(), "files.idempotency.lookup_failed", err)
		fail(w, 503, "Không kiểm tra được yêu cầu lặp.", "SERVICE_UNAVAILABLE")
		return nil, nil, true
	case found:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Idempotent-Replayed", "true")
		w.WriteHeader(status)
		w.Write(body)
		return nil, nil, true
	}
	return rec, func() {
		if rec.status < 200 || rec.status > 299 || !json.Valid(rec.body.Bytes()) {
			return
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
		defer cancel()
		if err := store.Remember(ctx, u.ID, keyHash, requestHash, rec.status, rec.body.Bytes()); err != nil {
			// The write is done; only replay protection is missing for this key.
			logError(r.Context(), "files.idempotency.store_failed", err)
		}
	}, false
}
