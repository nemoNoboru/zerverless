package volunteer

import (
	"testing"
)

func TestCapabilities_SupportsNamespace(t *testing.T) {
	c := Capabilities{
		Namespaces: []string{"alice", "bob"},
	}

	if !c.SupportsNamespace("alice") {
		t.Error("expected alice to be supported")
	}

	if !c.SupportsNamespace("bob") {
		t.Error("expected bob to be supported")
	}

	if c.SupportsNamespace("charlie") {
		t.Error("expected charlie to not be supported")
	}

	// Empty whitelist means all namespaces
	c2 := Capabilities{Namespaces: []string{}}
	if !c2.SupportsNamespace("any") {
		t.Error("empty whitelist should support all namespaces")
	}
}


