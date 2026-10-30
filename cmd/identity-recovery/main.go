// identity-recovery is a local OS-root tool, never an HTTP endpoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/identity"
)

func privateRead(path string) (string, error) {
	file, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return "", e
	}
	defer file.Close()
	info, e := file.Stat()
	if e != nil {
		return "", e
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return "", errors.New("root-owned private regular file required")
	}
	data, e := io.ReadAll(io.LimitReader(file, 4097))
	if len(data) > 4096 {
		return "", errors.New("private input too large")
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r"), e
}
func run() error {
	mode := flag.String("mode", "", "issue or consume")
	user := flag.String("user", "", "existing active username")
	token := flag.String("token-file", "", "private token path; create exclusively for issue")
	password := flag.String("password-file", "", "private new-password path for consume")
	flag.Parse()
	if os.Geteuid() != 0 {
		return errors.New("OS root required")
	}
	if (*mode != "issue" && *mode != "consume") || *user == "" || len(*user) > 64 || *token == "" || flag.NArg() != 0 {
		return errors.New("invalid local recovery arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		return errors.New("database configuration unavailable")
	}
	defer db.Close()
	core, e := identity.NewAuthCore(db)
	if e != nil {
		return e
	}
	if *mode == "issue" {
		if *password != "" {
			return errors.New("password-file only accepted for consume")
		}
		file, e := os.OpenFile(*token, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return errors.New("cannot create private token file exclusively")
		}
		defer file.Close()
		raw, e := core.IssueRecoveryLocal(ctx, *user)
		if e != nil {
			os.Remove(*token)
			return e
		}
		if _, e = file.WriteString(raw + "\n"); e != nil {
			return e
		}
		if e = file.Sync(); e != nil {
			return e
		}
		fmt.Println("recovery token saved privately; expires in 15 minutes; account unchanged")
		return nil
	}
	raw, e := privateRead(*token)
	if e != nil {
		return e
	}
	next, e := privateRead(*password)
	if e != nil {
		return e
	}
	if e = core.ConsumeRecoveryLocal(ctx, *user, raw, next); e != nil {
		return e
	}
	fmt.Println("password replaced; all Desktop and Drive sessions revoked; remove private input files")
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "local recovery failed; verify root, private files, active user and token expiry; no credential details logged")
		os.Exit(1)
	}
}
