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

// Package web embeds the HydraKVM browser-facing HTML index template and the
// bundled web-client static assets (compiled JS/CSS produced by
// `web/run.sh npm run build` into the `dist/` subdirectory). The `dist`
// subtree is gitignored except for a placeholder; sub-step 5 of Step 3
// populates it. Depends only on the standard library.
package web
