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

package auth

import (
	"context"
	"testing"
)

func TestNullProviderImplementsProvider(t *testing.T) {
	var p Provider = &NullProvider{}
	_ = p
}

func TestNullProviderAuthenticatePassesThroughUsername(t *testing.T) {
	p, err := NewNullProvider(nil)
	if err != nil {
		t.Fatalf("NewNullProvider: %v", err)
	}
	acct, err := p.Authenticate(context.Background(), "alice", "anything")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if acct == nil || acct.Username != "alice" {
		t.Errorf("got %+v, want Username=alice", acct)
	}
}

func TestNullProviderAuthorizePermits(t *testing.T) {
	p := &NullProvider{}
	ok, err := p.Authorize(context.Background(), &Account{Username: "alice"}, "anything")
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !ok {
		t.Error("NullProvider must permit any action")
	}
}
