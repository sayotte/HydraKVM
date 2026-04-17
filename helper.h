
// check if index is beyond the bounds of the given array
#define INDEX_OOB(arr, idx) ((idx) >= sizeof(arr) / sizeof((arr)[0]))

// convert ASCII string to UTF16-LE string (required by USB control protos)
void copyn_ascii_to_utf16LE(uint16_t *dst, char const *src, int count);
