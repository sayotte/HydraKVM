// Copyright (C) 2026 Stephen Ayotte
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package http

import (
	"testing"
	"time"
)

func TestTokenRegistryMintConsume(t *testing.T) {
	r := newTokenRegistry()
	tok, err := r.mint()
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if !r.consume(tok) {
		t.Error("consume immediately after mint should succeed")
	}
	if r.consume(tok) {
		t.Error("second consume should fail")
	}
}

func TestTokenRegistryExpiry(t *testing.T) {
	r := newTokenRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	tok, err := r.mint()
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	r.now = func() time.Time { return now.Add(tokenTTL + time.Second) }
	if r.consume(tok) {
		t.Error("expired token should not be consumable")
	}
}

func TestTokenRegistrySweepRemovesExpired(t *testing.T) {
	r := newTokenRegistry()
	now := time.Now()
	r.now = func() time.Time { return now }
	tok, _ := r.mint()
	r.now = func() time.Time { return now.Add(tokenTTL + time.Second) }
	r.sweep()
	r.mu.Lock()
	_, present := r.tokens[tok]
	r.mu.Unlock()
	if present {
		t.Error("sweep should have removed expired token")
	}
}

func TestValidToken(t *testing.T) {
	cases := map[string]bool{
		"":               false,
		"abc":            true,
		"abc-DEF_123":    true,
		"abc def":        false,
		"abc/def":        false,
		"abc=def":        false,
		"valid_token-99": true,
	}
	for in, want := range cases {
		if got := validToken(in); got != want {
			t.Errorf("validToken(%q) = %v want %v", in, got, want)
		}
	}
}
