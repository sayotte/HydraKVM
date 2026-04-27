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

// Package kvm defines the core domain model for HydraKVM: Channels (per-target
// hardware bindings), Clients (per-browser-tab connections), and the
// Application that owns the association between them. Drivers (picolink, v4l,
// synthetic) implement the interfaces declared here; they do not own domain
// state. See docs/code-map.md for the dependency rules; this package has no
// outbound internal imports and uses only the standard library (notably
// [context.Context] for cancellation propagation).
package kvm
