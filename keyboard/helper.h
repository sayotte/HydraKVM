/*
 * Copyright (C) 2026 Stephen Ayotte
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

// check if index is beyond the bounds of the given array
#define INDEX_OOB(arr, idx) ((idx) >= sizeof(arr) / sizeof((arr)[0]))

// convert ASCII string to UTF16-LE string (required by USB control protos)
void copyn_ascii_to_utf16LE(uint16_t *dst, char const *src, int count);
