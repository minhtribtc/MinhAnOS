package storagecore

import (
	"context"
	"testing"
)

// notices returns the queued quota notices of one recipient, oldest first.
func notices(t *testing.T, s Store, recipient string) (kinds []string, sources []string, params []map[string]any) {
	t.Helper()
	rows, e := s.DB.Query(context.Background(), "SELECT kind,source,details FROM storage_notification_outbox WHERE recipient_id=$1 ORDER BY created_at,id", recipient)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	for rows.Next() {
		var k, src string
		var p map[string]any
		if e = rows.Scan(&k, &src, &p); e != nil {
			t.Fatal(e)
		}
		kinds, sources, params = append(kinds, k), append(sources, src), append(params, p)
	}
	return
}

func TestQuotaThresholdNotices(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	const owner, admin = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	if _, e := s.DB.Exec(ctx, "INSERT INTO users(id,public_id,username,display_name,status,role) VALUES('home-n',$1,'nora','Nora','active','user'),('adm',$2,'admin','Admin','active','admin')", owner, admin); e != nil {
		t.Fatal(e)
	}
	limit := 100 * mib
	if e := (Manager{Store: s}).SetDefaultHomeLimit(ctx, "", 1, &limit); e != nil {
		t.Fatal(e)
	}
	write := func(bytes int64) {
		t.Helper()
		r, e := s.Reserve(ctx, "home-n", bytes)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Commit(ctx, r, bytes); e != nil {
			t.Fatal(e)
		}
	}
	write(50 * mib)
	if k, _, _ := notices(t, s, owner); len(k) != 0 {
		t.Fatal("notice below 80 %", k)
	}
	write(31 * mib) // 81 %
	write(1 * mib)  // still the same level: no second notice
	write(14 * mib) // 96 %
	write(4 * mib)  // 100 %
	kinds, sources, params := notices(t, s, owner)
	if len(kinds) != 3 || kinds[0] != "quota.warning" || kinds[1] != "quota.critical" || kinds[2] != "quota.full" {
		t.Fatal("threshold notices", kinds)
	}
	if sources[0] != "com.minhanos.files" || params[0]["scope"] != "home" || params[0]["percent"] != float64(81) {
		t.Fatal("home notice", sources[0], params[0])
	}
	if _, ok := params[0]["path"]; ok {
		t.Fatal("home notice carries a path")
	}
	// Falling below a level lowers it silently; crossing it again notifies again.
	if e := s.SetMeasured(ctx, "home-n", 10*mib); e != nil {
		t.Fatal(e)
	}
	if k, _, _ := notices(t, s, owner); len(k) != 3 {
		t.Fatal("notice on falling usage", k)
	}
	write(75 * mib) // 85 %
	if k, _, _ := notices(t, s, owner); len(k) != 4 || k[3] != "quota.warning" {
		t.Fatal("re-crossing", k)
	}
	// A Shared Folder notifies the active System Admins with the share name.
	if _, e := s.DB.Exec(ctx, "UPDATE storage_quota SET limit_bytes=used_bytes WHERE scope_type='share'"); e != nil {
		t.Fatal(e)
	}
	kinds, sources, params = notices(t, s, admin)
	if len(kinds) != 1 || kinds[0] != "quota.full" || sources[0] != "com.minhanos.storage" || params[0]["scope"] != "share" || params[0]["share"] == nil {
		t.Fatal("share notice", kinds, sources, params)
	}
	var tagged int
	if e := s.DB.QueryRow(ctx, "SELECT count(*) FROM storage_notification_outbox WHERE request_id=''").Scan(&tagged); e != nil || tagged != 0 {
		t.Fatal("empty request id", tagged, e)
	}
}
