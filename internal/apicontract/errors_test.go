package apicontract

import "testing"

func TestCodesIndependentOfLegacyText(t *testing.T) {
	a, b := Error(403, "Vietnamese"), Error(403, "English")
	if a["code"] != "ACCESS_DENIED" || a["code"] != b["code"] {
		t.Fatal(a, b)
	}
	if Error(401, "legacy", "INVALID_CREDENTIALS")["code"] != "INVALID_CREDENTIALS" {
		t.Fatal("override")
	}
}
