package deploy

import (
	"testing"
)

func TestNewDeployment(t *testing.T) {
	d := New("adamska", "/hello", "lua", `function handle(r) return {status=200} end`)

	if d.User != "adamska" {
		t.Errorf("expected adamska, got %s", d.User)
	}
	if d.Path != "/hello" {
		t.Errorf("expected /hello, got %s", d.Path)
	}
	if d.Runtime != "lua" {
		t.Errorf("expected lua, got %s", d.Runtime)
	}
	if d.CreatedAt.IsZero() {
		t.Error("expected created_at")
	}
}

func TestStore_SetAndGet(t *testing.T) {
	store := NewStore()
	d := New("adamska", "/hello", "lua", "code")

	if err := store.Set(d); err != nil {
		t.Fatalf("set deployment: %v", err)
	}
	got, err := store.Get("adamska", "/hello")

	if err != nil {
		t.Fatalf("expected deployment to exist: %v", err)
	}
	if got.Code != "code" {
		t.Errorf("expected code, got %s", got.Code)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := NewStore()

	_, err := store.Get("nobody", "/nothing")
	if err == nil {
		t.Error("expected deployment not found")
	}
}

func TestStore_Overwrite(t *testing.T) {
	store := NewStore()

	if err := store.Set(New("adamska", "/hello", "lua", "v1")); err != nil {
		t.Fatalf("set deployment: %v", err)
	}
	if err := store.Set(New("adamska", "/hello", "lua", "v2")); err != nil {
		t.Fatalf("set deployment: %v", err)
	}

	got, _ := store.Get("adamska", "/hello")
	if got.Code != "v2" {
		t.Errorf("expected v2, got %s", got.Code)
	}
}
