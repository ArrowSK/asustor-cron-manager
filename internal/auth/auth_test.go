package auth

import (
	"path/filepath"
	"testing"
)

func TestAuthLifecycle(t *testing.T) {
	m := New(filepath.Join(t.TempDir(), "auth.json"))
	if !m.SetupRequired() {
		t.Fatal("expected setup")
	}
	if err := m.Setup("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if m.SetupRequired() {
		t.Fatal("setup should be complete")
	}
	if !m.Verify("correct horse battery staple") {
		t.Fatal("verify failed")
	}
	if m.Verify("wrong password") {
		t.Fatal("wrong password accepted")
	}
	token, _, err := m.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if !m.ValidSession(token) {
		t.Fatal("session invalid")
	}
	m.Revoke(token)
	if m.ValidSession(token) {
		t.Fatal("session should be revoked")
	}
}
