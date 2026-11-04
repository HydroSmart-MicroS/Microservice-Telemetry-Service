package transform

import (
	"fmt"
	"testing"
)

func TestRejectsMissingFieldsAndSpoofedExtensions(t *testing.T) {
	base := `{"schema_version":1,"reading_id":"00000000-0000-4000-8000-000000000001","device_id":"00000000-0000-4000-8000-000000000002","boot_id":"00000000-0000-4000-8000-000000000003","sequence":0,"volume_ml":0,"interval_seconds":1,"measured_at":"2026-10-24T00:00:00Z"%s}`
	if _, err := ToCommand("topic", []byte(fmt.Sprintf(base, ""))); err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{`,"unit_id":"forged"`, `,"owner_user_id":"forged"`} {
		if _, err := ToCommand("topic", []byte(fmt.Sprintf(base, extension))); err == nil {
			t.Fatal("unexpected fields accepted")
		}
	}
	for _, payload := range []string{`{}`, `{"schema_version":1}`, fmt.Sprintf(base, "") + " {}", `{"schema_version":1,"sequence":null}`} {
		if _, err := ToCommand("topic", []byte(payload)); err == nil {
			t.Fatal("incomplete or multiple documents accepted")
		}
	}
}
