package coreprofile

import "testing"

func TestValidation(t *testing.T) {
	good := Profile{"vi-VN", "system", "Asia/Bangkok"}
	if !Valid(good) {
		t.Fatal("valid profile rejected")
	}
	for _, p := range []Profile{{"fr-FR", "light", "UTC"}, {"en-US", "invalid", "UTC"}, {"en-US", "dark", "no/such-zone"}, {"en-US", "light", ""}} {
		if Valid(p) {
			t.Fatalf("accepted invalid profile: %+v", p)
		}
	}
}
