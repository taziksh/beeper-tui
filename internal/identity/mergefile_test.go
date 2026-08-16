package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergePolicyRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity-merges.yaml")

	policy, err := LoadMergePolicy(path)
	if err != nil {
		t.Fatalf("LoadMergePolicy(missing) error = %v", err)
	}
	if policy.allowNameMerge("John Smith", []string{"wa", "ig"}) {
		t.Fatal("common name merged without approval")
	}
	if err := SaveMergePolicy(path, policy); err != nil {
		t.Fatalf("SaveMergePolicy: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	if !strings.Contains(string(data), "john smith") {
		t.Errorf("sidecar = %q, want pending john smith", data)
	}

	edited := strings.Replace(string(data), "approved: []", "approved:\n  - john smith", 1)
	if edited == string(data) {
		t.Fatalf("sidecar = %q, want an empty approved list to edit", data)
	}
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err = LoadMergePolicy(path)
	if err != nil {
		t.Fatalf("LoadMergePolicy(edited) error = %v", err)
	}
	if !policy.allowNameMerge("John Smith", nil) {
		t.Error("approved name did not merge")
	}
}

func TestSaveMergePolicySkipsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity-merges.yaml")
	if err := SaveMergePolicy(path, &MergePolicy{}); err != nil {
		t.Fatalf("SaveMergePolicy: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("empty policy wrote a file")
	}
}
