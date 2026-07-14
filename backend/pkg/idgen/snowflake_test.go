package idgen

import "testing"

func TestGenerator_Uniqueness(t *testing.T) {
	g := New(1)
	seen := make(map[int64]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := g.Next()
		if seen[id] {
			t.Fatalf("duplicate id: %d", id)
		}
		seen[id] = true
	}
}

func TestDirectConversationID(t *testing.T) {
	if DirectConversationID(5, 3) != "3_5" {
		t.Fatal("expected sorted pair")
	}
}
