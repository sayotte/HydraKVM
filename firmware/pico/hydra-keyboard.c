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

#include <stdio.h>

#include "hardware/uart.h"
#include "hardware/irq.h"
#include "hardware/sync.h"
#include "hardware/watchdog.h"
#include "pico/multicore.h"
#include "pico/stdlib.h"
#include "pico/util/queue.h"
#include "bsp/board.h"
#include "tusb.h"

#define HID_REPORT_TIMEOUT_US  10000   // 10ms — ~10 USB frames
#define WATCHDOG_TIMEOUT_MS    5000
#define UART_BAUDRATE          115200
#define CMD_QUEUE_DEPTH        64 // in a 10ms timeout we'll accumulate ~38 commands

static queue_t cmd_queue;

// Wire protocol: every command is 3 bytes: [0xFF, mod, kc].
// 0xFF is never a valid keycode (reserved in HID usage page 0x07)
// and is effectively impossible as a mod byte (all 8 modifiers held
// at once). Any stray 0xFF re-syncs the receiver, so a dropped or
// garbled byte costs at most one command.
#define SYNC_BYTE 0xFF

// Anomaly counters. Inspect via debugger or surface over uart1 (debug)
static volatile uint32_t n_hid_timeouts;
static volatile uint32_t n_discarded_unmounted;
static volatile uint32_t n_resyncs;          // unexpected SYNC mid-frame
static volatile uint32_t n_pre_sync_bytes;   // bytes arriving before first SYNC

void on_uart_rx_isr() {
    enum { WAIT_SYNC, WAIT_MOD, WAIT_KC };
    static uint8_t state = WAIT_SYNC;
    static uint16_t cmd;

    while (uart_is_readable(uart0)) {
        uint8_t b = (uint8_t)uart_getc(uart0);

        if (b == SYNC_BYTE) {
            if (state != WAIT_SYNC) n_resyncs++;
            state = WAIT_MOD;
            continue;
        }

        switch (state) {
            case WAIT_SYNC:
                // Desynced — drop bytes until the next SYNC arrives.
                n_pre_sync_bytes++;
                break;
            case WAIT_MOD:
                cmd = (uint16_t)b << 8;
                state = WAIT_KC;
                break;
            case WAIT_KC:
                cmd |= b;
                queue_try_add(&cmd_queue, &cmd);
                state = WAIT_SYNC;
                break;
        }
    }
}

void wire_uart() {
    uart_init(uart0, UART_BAUDRATE);
    gpio_set_function(0, GPIO_FUNC_UART); // TX on GPIO-0 
    gpio_set_function(1, GPIO_FUNC_UART); // RX on GPIO-1
    gpio_pull_up(1);

    irq_set_exclusive_handler(UART0_IRQ, on_uart_rx_isr);
    irq_set_enabled(UART0_IRQ, true);
    uart_set_irq_enables(uart0, true, false); // RX interrupt on, TX off
}

void core1_main() {
    wire_uart();
}

// Poll tud_task() until the HID IN endpoint is free or the deadline
// expires. Bounded wait so a dead/stalled host can't wedge us forever.
static bool hid_ready_wait(uint32_t timeout_us) {
    absolute_time_t deadline = make_timeout_time_us(timeout_us);
    while (!tud_hid_ready()) {
        tud_task();
        if (time_reached(deadline)) return false;
    }
    return true;
}

static void drain_cmd_queue(void) {
    uint16_t discard;
    while (queue_try_remove(&cmd_queue, &discard)) {
        n_discarded_unmounted++;
    }
}

void usb_loop_once() {
    tud_task();

    // If the bus isn't usable, discard queued input instead of letting
    // it pile up and replay stale keystrokes when the host returns.
    if (!tud_mounted() || tud_suspended()) {
        drain_cmd_queue();
    }
    else {
        uint16_t cmd;
        if (!queue_try_remove(&cmd_queue, &cmd)) {
            return;
        }

        uint8_t mod = (cmd >> 8) & 0xFF;
        uint8_t kc  = cmd & 0xFF;

        if (hid_ready_wait(HID_REPORT_TIMEOUT_US)) {
            uint8_t keys[6] = {kc, 0, 0, 0, 0, 0};
            tud_hid_keyboard_report(0, mod, keys);
        }
        else {
            n_hid_timeouts++;
        }

        if (hid_ready_wait(HID_REPORT_TIMEOUT_US)) {
            tud_hid_keyboard_report(0, 0, NULL);
        }
        else {
            n_hid_timeouts++;
        }
    }

    // Sleep until there's work to do. WFE wakes on cross-core SEV
    // (queue_try_add on core 1) or any enabled IRQ going pending
    // (USB IRQ on this core), via SCR.SEVONPEND.
    if (!tud_task_event_ready() && queue_is_empty(&cmd_queue)) {
        __wfe();
    }
}

// Do nothing when the timer fires-- it exists merely to trigger an interrupt
// to wake us up occasionally so the watchdog doesn't kill us.
static bool liveness_tick(repeating_timer_t *t) { (void)t; return true; }

int main(void) {
    board_init();
    stdio_init_all();
    tusb_init();
    queue_init(&cmd_queue, sizeof(uint16_t), CMD_QUEUE_DEPTH);

    watchdog_enable(WATCHDOG_TIMEOUT_MS, true);  // true == pause on debug halt
    // Wake 1x/sec, allowing us to loop if we're waiting on events/interrupts
    // This prevents the watchdog from rebooting us needlessly, but if we're
    // stuck on anything else this won't break us free and the watchdog will
    // appropriately fire.
    static repeating_timer_t liveness_timer;
    add_repeating_timer_ms(1000, liveness_tick, NULL, &liveness_timer);

    multicore_launch_core1(core1_main);

    while(true) {
        watchdog_update();
        usb_loop_once();
    }

    return 0;
}

// On (re)connect, flush any phantom key left pressed by a partial
// send before disconnect, and discard stale queued input.
void tud_mount_cb(void) {
    drain_cmd_queue();
    if (tud_hid_ready()) tud_hid_keyboard_report(0, 0, NULL);
}

void tud_umount_cb(void) {
    drain_cmd_queue();
}

// TinyUSB callbacks we don't use but must provide
uint16_t tud_hid_get_report_cb(uint8_t i, uint8_t id, hid_report_type_t t,
                                uint8_t* buf, uint16_t len) {
    (void)i; (void)id; (void)t; (void)buf; (void)len; return 0;
}
void tud_hid_set_report_cb(uint8_t i, uint8_t id, hid_report_type_t t,
                            uint8_t const* buf, uint16_t len) {
    (void)i; (void)id; (void)t; (void)buf; (void)len;
}
