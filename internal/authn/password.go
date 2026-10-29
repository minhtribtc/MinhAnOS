// Package authn contains identity primitives shared by independently runnable apps.
package authn

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

const Iterations = 600000

func Hash(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, Iterations, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(key), nil
}
func Verify(password, hash string) bool {
	p := strings.Split(hash, ":")
	if len(p) != 2 {
		return false
	}
	salt, e1 := hex.DecodeString(p[0])
	want, e2 := hex.DecodeString(p[1])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(want) != 32 {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, Iterations, 32)
	return err == nil && subtle.ConstantTimeCompare(key, want) == 1
}
func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Digest(value string) string { h := sha256.Sum256([]byte(value)); return hex.EncodeToString(h[:]) }
