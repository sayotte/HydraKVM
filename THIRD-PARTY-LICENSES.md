# Third-party licenses

HydraKVM incorporates code from the projects listed below. Each is covered by
its own license; the terms of those licenses apply to the corresponding
upstream code regardless of HydraKVM's own dual-licensing scheme (see
[`LICENSING.md`](LICENSING.md)).

This file is informational and must accompany binary distributions of
HydraKVM (e.g. `hydra-keyboard.uf2`, any compiled HydraKVM server binary,
or any commercial product incorporating HydraKVM) to satisfy the
notice-preservation requirements of the upstream permissive licenses.

> **Verification note.** The copyright notices below are reproduced from
> upstream as a starting point. Before any binary release, check the
> upstream `LICENSE` file of each dependency and update copyright dates and
> holders to match exactly what those files say at the version being shipped.

---

## Raspberry Pi Pico SDK

- **Project:** Raspberry Pi Pico SDK
- **URL:** https://github.com/raspberrypi/pico-sdk
- **License:** BSD-3-Clause
- **Used in:** firmware build system and runtime — `firmware/pico/CMakeLists.txt`,
  `firmware/pico/pico_sdk_import.cmake`, all of `firmware/pico/*.c` (linked against
  `pico_stdlib`, `pico_multicore`, etc.)

Copyright notice (reproduced from the upstream `LICENSE.TXT`):

> Copyright 2020 (c) 2020 Raspberry Pi (Trading) Ltd.

License text: see [BSD-3-Clause](#bsd-3-clause-license-text) below.

---

## TinyUSB

- **Project:** TinyUSB
- **URL:** https://github.com/hathach/tinyusb
- **License:** MIT
- **Used in:** firmware USB stack — linked into `firmware/pico/hydra-keyboard.c`
  via `tinyusb_device` / `tinyusb_board`. `firmware/pico/usb_descriptors.c` was
  derived in part from TinyUSB's HID keyboard example.

Copyright notice (reproduced from upstream):

> Copyright (c) 2018, hathach (tinyusb.org)

License text: see [MIT](#mit-license-text) below.

The MIT notice above also covers the portions of `firmware/pico/usb_descriptors.c`
that were derived from TinyUSB example code.

---

## bugst/go-serial (`go.bug.st/serial`)

- **Project:** bugst/go-serial
- **URL:** https://github.com/bugst/go-serial
- **License:** BSD-3-Clause
- **Used in:** Go server — `server/server.go` opens the Pico's serial port
  through this library.

Copyright notice (reproduced from upstream `LICENSE`):

> Copyright (c) 2014-2023, Cristian Maglie.
> All rights reserved.

License text: see [BSD-3-Clause](#bsd-3-clause-license-text) below.

---

## golang.org/x/term

- **Project:** Go Authors' supplementary `term` package
- **URL:** https://pkg.go.dev/golang.org/x/term
- **License:** BSD-3-Clause (the standard "Go authors" variant)
- **Used in:** Go server — `server/server.go` uses it to put stdin into
  raw mode for the CLI keystroke loop.

Copyright notice (reproduced from upstream `LICENSE`):

> Copyright (c) 2009 The Go Authors. All rights reserved.

License text: see [BSD-3-Clause](#bsd-3-clause-license-text) below.

---

## BSD-3-Clause license text

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its
   contributors may be used to endorse or promote products derived from
   this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.

---

## MIT license text

Permission is hereby granted, free of charge, to any person obtaining a
copy of this software and associated documentation files (the "Software"),
to deal in the Software without restriction, including without limitation
the rights to use, copy, modify, merge, publish, distribute, sublicense,
and/or sell copies of the Software, and to permit persons to whom the
Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
DEALINGS IN THE SOFTWARE.

---

## How to update this file

When adding a new dependency:

1. Identify its license (check the repo's `LICENSE` file, `go.sum`, or
   the package metadata).
2. Add a section above with the project name, URL, license, copyright
   notice as it appears upstream, and a brief note about where in
   HydraKVM it is used.
3. If the license is BSD-3-Clause or MIT, no new license-text section is
   needed — point to the existing one.
4. If the license is something new (Apache-2.0, ISC, BSD-2-Clause, etc.),
   add a new license-text section at the bottom.
5. If a dependency is **copyleft** (GPL, LGPL, MPL, AGPL, etc.), stop and
   flag for review. Copyleft dependencies may break the commercial
   dual-licensing model in [`LICENSING.md`](LICENSING.md), since you
   cannot grant a non-copyleft commercial license over code you do not
   own.
