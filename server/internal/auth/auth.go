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
	"encoding/json"
)

// Account represents an authenticated principal.
type Account struct {
	Username string
}

// Authenticator validates a credential and returns the corresponding Account.
type Authenticator interface {
	Authenticate(ctx context.Context, username, password string) (*Account, error)
}

// Authorizer decides whether an Account may perform a named action.
type Authorizer interface {
	Authorize(ctx context.Context, acct *Account, action string) (bool, error)
}

// Provider combines [Authenticator] and [Authorizer]. Concrete providers (a
// local password file in Step 7, etc.) implement this.
type Provider interface {
	Authenticator
	Authorizer
}

// NullProvider is a stub Provider that accepts any credential and authorizes
// any action. Used for Step 2 wiring and tests; it must not ship in a
// production build configuration.
type NullProvider struct{}

// NewNullProvider constructs a [NullProvider]. The cfg blob is accepted for
// configuration-shape compatibility and is otherwise ignored.
func NewNullProvider(_ json.RawMessage) (*NullProvider, error) {
	return &NullProvider{}, nil
}

// Authenticate returns an Account for any non-empty username.
func (p *NullProvider) Authenticate(_ context.Context, username, _ string) (*Account, error) {
	return &Account{Username: username}, nil
}

// Authorize permits any action.
func (p *NullProvider) Authorize(_ context.Context, _ *Account, _ string) (bool, error) {
	return true, nil
}
