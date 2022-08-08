package main

import "testing"

func TestGenerateToken(t *testing.T) {
	pool := make(map[string]bool)
	for i := 0; i < 1024; i++ {
		c := GenerateToken()
		if len(c) != 21 {
			t.Fatalf("Token has wrong length. Token %s, length: %d", c, len(c))
		}
		if _, ok := pool[c]; ok {
			t.Fatalf("Token repeated, after %d iterations", i)
		}
		pool[c] = true
	}

	t.Log("Many tokens", pool)
}
