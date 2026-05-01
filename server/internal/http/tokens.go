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
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/sayotte/hydrakvm/internal/kvm"
)

const tokenTTL = 30 * time.Second

type tokenRegistry struct {
	mu     sync.Mutex
	tokens map[string]time.Time
	now    func() time.Time
}

func newTokenRegistry() *tokenRegistry {
	return &tokenRegistry{
		tokens: make(map[string]time.Time),
		now:    time.Now,
	}
}

func (r *tokenRegistry) mint() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	tok := base64.RawURLEncoding.EncodeToString(buf[:])
	r.mu.Lock()
	r.tokens[tok] = r.now()
	r.mu.Unlock()
	return tok, nil
}

func (r *tokenRegistry) consume(token string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	issued, ok := r.tokens[token]
	if !ok {
		return false
	}
	delete(r.tokens, token)
	return r.now().Sub(issued) <= tokenTTL
}

func (r *tokenRegistry) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
}

func (r *tokenRegistry) sweepLocked() {
	cutoff := r.now().Add(-tokenTTL)
	for tok, issued := range r.tokens {
		if issued.Before(cutoff) {
			delete(r.tokens, tok)
		}
	}
}

// streamRegistry holds a token -> *kvm.Client mapping for the /stream/{token}
// endpoint. Tokens are minted by the WebSocket handler after a Client is
// registered with the Application; the stream handler looks the Client up to
// install its mjpegSink as VideoOut. The token is valid for the lifetime of
// the WS connection and may be resolved repeatedly (browsers re-fetch the
// MJPEG stream when video.src is reassigned across channel switches /
// failover); release sweeps it on Client removal.
type streamRegistry struct {
	mu     sync.Mutex
	tokens map[string]*kvm.Client
}

func newStreamRegistry() *streamRegistry {
	return &streamRegistry{tokens: make(map[string]*kvm.Client)}
}

func (r *streamRegistry) mint(c *kvm.Client) (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	tok := base64.RawURLEncoding.EncodeToString(buf[:])
	r.mu.Lock()
	r.tokens[tok] = c
	r.mu.Unlock()
	return tok, nil
}

func (r *streamRegistry) consume(token string) (*kvm.Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.tokens[token]
	return c, ok
}

// release removes any pending token for c without consuming it (e.g. on WS
// disconnect before /stream/ is hit).
func (r *streamRegistry) release(c *kvm.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tok, cl := range r.tokens {
		if cl == c {
			delete(r.tokens, tok)
		}
	}
}

// validToken returns true iff s is non-empty and contains only base64url
// alphabet characters. Cheap pre-check before touching the registry map.
func validToken(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}
