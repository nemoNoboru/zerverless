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

	store.Set(d)
	got, ok := store.Get("adamska", "/hello")

	if !ok {
		t.Fatal("expected deployment to exist")
	}
	if got.Code != "code" {
		t.Errorf("expected code, got %s", got.Code)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := NewStore()

	_, ok := store.Get("nobody", "/nothing")
	if ok {
		t.Error("expected deployment not found")
	}
}

func TestStore_Overwrite(t *testing.T) {
	store := NewStore()

	store.Set(New("adamska", "/hello", "lua", "v1"))
	store.Set(New("adamska", "/hello", "lua", "v2"))

	got, _ := store.Get("adamska", "/hello")
	if got.Code != "v2" {
		t.Errorf("expected v2, got %s", got.Code)
	}
}

