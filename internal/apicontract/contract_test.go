package apicontract

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestLegacyWireContract(t *testing.T) {
	raw, e := os.ReadFile("../../contracts/fixtures/error.legacy.json")
	if e != nil {
		t.Fatal(e)
	}
	var expected map[string]string
	if e = json.Unmarshal(raw, &expected); e != nil {
		t.Fatal(e)
	}
	actual := Error(403, "legacy message")
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("legacy contract changed: %#v", actual)
	}
}
