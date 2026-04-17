#ifndef RINGBUF_H
#define RINGBUF_H

#include <stdint.h>
#include <stdbool.h>

#define RINGBUF_SIZE 128  // must be power of 2

typedef struct {
    volatile uint32_t head;  // written by producer
    volatile uint32_t tail;  // written by consumer
    char buf[RINGBUF_SIZE];
} ringbuf_t;

void ringbuf_init(ringbuf_t *rb);
bool ringbuf_empty(ringbuf_t *rb);
bool ringbuf_full(ringbuf_t *rb);
bool ringbuf_write(ringbuf_t *rb, char c);
bool ringbuf_read(ringbuf_t *rb, char *c);

#endif
