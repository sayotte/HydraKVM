#include <stdint.h>
#include <stddef.h>

void copyn_ascii_to_utf16LE(uint16_t *dst, char const *src, size_t count) {
    for ( size_t i = 0; i < count; i++ ) {
        dst[i] = src[i];
    }
}
