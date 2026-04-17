#include <stdio.h>

#include "hardware/uart.h"
#include "hardware/irq.h"
#include "hardware/sync.h"
#include "pico/multicore.h"
#include "pico/stdlib.h"
#include "bsp/board.h"
#include "tusb.h"

#include "ringbuf.h"

static ringbuf_t uart_rx_buf;

void on_uart_rx_isr() {
    while(uart_is_readable(uart0)) {
        char ch = uart_getc(uart0);
        ringbuf_write(&uart_rx_buf, ch);
    }
}

void wire_uart() {
    ringbuf_init(&uart_rx_buf);

    uart_init(uart0, 115200);
    gpio_set_function(0, GPIO_FUNC_UART); // TX on GPIO-0 
    gpio_set_function(1, GPIO_FUNC_UART); // RX on GPIO-1
    gpio_pull_up(1);

    irq_set_exclusive_handler(UART0_IRQ, on_uart_rx_isr);
    irq_set_enabled(UART0_IRQ, true);
    uart_set_irq_enables(uart0, true, false); // RX interrupt on, TX off
}

void core1_main() {
    wire_uart();
    uint8_t first_byte;
    bool have_first = false;

    while(true) {
        char ch;
        if(ringbuf_read(&uart_rx_buf, &ch)) {
            if (!have_first) {
                first_byte = (uint8_t)ch;
                have_first = true;
            } else {
                uint32_t cmd = ((uint32_t)first_byte << 8) | (uint8_t)ch;
                multicore_fifo_push_blocking(cmd);
                have_first = false;
            }
        } else {
            // If no outstanding work, wait for any interrupt to wake us up
            //
            // Disable interrupts as we do it-- avoids a race between ISR writing data
            // between us seeing the ringbuf is empty and calling __wfi(). When we wake
            // from __wfi() the interrupt bit will be set, so as soon as we enable the
            // interrupt the ISR will immediately fire.
            uint32_t saved_ints = save_and_disable_interrupts();
            if(ringbuf_empty(&uart_rx_buf)) {
                __wfi();
            }
            restore_interrupts(saved_ints);
        }        
    }
}

void on_sio_core0_isr() {
    multicore_fifo_clear_irq();
}

void usb_loop_once() {
    tud_task();

    if (multicore_fifo_rvalid()) {
        uint32_t cmd = multicore_fifo_pop_blocking();
        uint8_t mod = (cmd >> 8) & 0xFF;
        uint8_t kc  = cmd & 0xFF;

        // Press
        while (!tud_hid_ready()) { tud_task(); }
        uint8_t keys[6] = {kc, 0, 0, 0, 0, 0};
        tud_hid_keyboard_report(0, mod, keys);

        // Release
        while (!tud_hid_ready()) { tud_task(); }
        tud_hid_keyboard_report(0, 0, NULL);
    }

    // like the UART core-- sleep until an interrupt wakes us
/*     uint32_t saved_ints = save_and_disable_interrupts();
    if(!tud_task_event_ready() && !multicore_fifo_rvalid()) {
        __wfi();
    }
    restore_interrupts(saved_ints); */

    return;
}

int main(void) {
    board_init();
    stdio_init_all();
    tusb_init();
    multicore_launch_core1(core1_main);

/*     while(!tud_mounted()) {
        tud_task();
        sleep_ms(50);
        printf("usb: waiting for tud_mounted() to be true\n");
    } */

 /*    irq_set_exclusive_handler(SIO_IRQ_PROC0, on_sio_core0_isr);
    irq_set_enabled(SIO_IRQ_PROC0, true); */
    while(true) {
        usb_loop_once();
    }

    return 0;
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
