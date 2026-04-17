#include "ringbuf.h"

void ringbuf_init(ringbuf_t *rb) {
    rb->head = 0;
    rb->tail = 0;
}

bool ringbuf_empty(ringbuf_t *rb) {
    return rb->head == rb->tail;
}

bool ringbuf_full(ringbuf_t *rb) {
    return ((rb->head + 1) & (RINGBUF_SIZE - 1)) == rb->tail;
}

bool ringbuf_write(ringbuf_t *rb, char c) {
    if (ringbuf_full(rb)) return false;
    rb->buf[rb->head] = c;
    rb->head = (rb->head + 1) & (RINGBUF_SIZE - 1);
    return true;
}

bool ringbuf_read(ringbuf_t *rb, char *c) {
    if (ringbuf_empty(rb)) return false;
    *c = rb->buf[rb->tail];
    rb->tail = (rb->tail + 1) & (RINGBUF_SIZE - 1);
    return true;
}
