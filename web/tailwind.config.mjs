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

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './src/**/*.{html,ts}',
    '../server/internal/http/web/index.html.tmpl',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        ink: '#1a1917',
        charcoal: '#3b3632',
        slate: '#685f58',
        warmGray: '#97918a',
        stone: '#c5beb4',
        sand: '#e8e2d9',
        linen: '#f2ede6',
        paper: '#f9f6f1',
        accent: '#a0705a',
        accentLt: '#b8836a',
        darkBg: '#1c1a18',
        darkSurface: '#242120',
        darkElevated: '#2a2725',
        darkBorder: '#3a3633',
      },
      fontFamily: {
        body: ['Inter', 'system-ui', 'sans-serif'],
        display: ['"Source Serif 4"', 'Georgia', 'serif'],
        mono: ['"JetBrains Mono"', '"Fira Code"', 'Consolas', 'monospace'],
      },
    },
  },
  plugins: [],
};
