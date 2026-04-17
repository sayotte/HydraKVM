#include "tusb.h"

#include "helper.h"

#define USB_VID   0xCafe   // replace before distributing
#define USB_PID   0x4004
#define USB_BCD   0x0200

tusb_desc_device_t const desc_device = {
    .bLength            = sizeof(tusb_desc_device_t),
    .bDescriptorType    = TUSB_DESC_DEVICE,
    .bcdUSB             = USB_BCD,
    .bDeviceClass       = 0,
    .bDeviceSubClass    = 0,
    .bDeviceProtocol    = 0,
    .bMaxPacketSize0    = CFG_TUD_ENDPOINT0_SIZE,
    .idVendor           = USB_VID,
    .idProduct          = USB_PID,
    .bcdDevice          = 0x0100,
    .iManufacturer      = 0x01,
    .iProduct           = 0x02,
    .iSerialNumber      = 0x03,
    .bNumConfigurations = 0x01,
};

uint8_t const * tud_descriptor_device_cb(void) {
    return (uint8_t const *) &desc_device;
}

//--------------------------------------------------------------------+
// HID Report Descriptor
//--------------------------------------------------------------------+

uint8_t const desc_hid_report[] = {
    // report that we're a keyboard-- we send reports with:
    // - 1 modifier byte
    // - 1 reserve byte
    // - 6 keycodes
    TUD_HID_REPORT_DESC_KEYBOARD(
        /* commented because we have only a single device / single report channel
        HID_REPORT_ID(1)
        */
    )
};

// Invoked when received GET HID REPORT DESCRIPTOR
// Application return pointer to descriptor
// Descriptor contents must exist long enough for transfer to complete
uint8_t const * tud_hid_descriptor_report_cb(uint8_t instance) {
    (void) instance;
    return desc_hid_report;
}

//--------------------------------------------------------------------+
// Configuration Descriptor
//--------------------------------------------------------------------+

#define CONFIG_TOTAL_LEN  (TUD_CONFIG_DESC_LEN + TUD_HID_DESC_LEN)

// 0x01 (endpoint number 1) & 0x80 (direction: IN, i.e. device->host)
#define EPNUM_HID         0x81 

uint8_t const desc_configuration[] = {
    TUD_CONFIG_DESCRIPTOR(
        // Config number
        1,
        // interface count
        1,
        // string index
        0,
        // total length
        CONFIG_TOTAL_LEN,
        // attribute
        TUSB_DESC_CONFIG_ATT_REMOTE_WAKEUP,
        // power in mA
        50
    ),
    TUD_HID_DESCRIPTOR(
        // interface number
        0,
        // string index
        0,
        // protocol
        HID_ITF_PROTOCOL_KEYBOARD,
        // report descriptor len
        sizeof(desc_hid_report),
        // EP In address
        EPNUM_HID,
        // size 
        CFG_TUD_HID_EP_BUFSIZE,
        // polling interval
        10
    ),
};

uint8_t const * tud_descriptor_configuration_cb(uint8_t index) {
    (void) index;
    return desc_configuration;
}

//--------------------------------------------------------------------+
// String Descriptors
//--------------------------------------------------------------------+

char const *string_desc_arr[] = {
    // language ID
    (const char[]){0x09, 0x04},   // English (US)
    // manufacturer
    "Needlessly Complex",
    // product
    "Virtual Keyboard",
    // serial
    NULL,
};

static uint16_t _desc_str[32];

// Invoked when received GET STRING DESCRIPTOR request
// Application return pointer to descriptor, whose contents must exist long enough for transfer to complete
uint16_t const *tud_descriptor_string_cb(uint8_t index, uint16_t langid) {
    (void) langid;
    uint8_t chr_count;

    if (index == 0) {
        _desc_str[1] = 0x0409; // USB Language ID: English (US)
        chr_count = 1;
    } else {
        if (INDEX_OOB(string_desc_arr, index)) return NULL;
        const char* str = string_desc_arr[index];

        // Cap at max char
        chr_count = strlen(str);
        if (chr_count > 31) chr_count = 31;

        copyn_ascii_to_utf16LE(&_desc_str[1], str, chr_count);
    }
    _desc_str[0] = (TUSB_DESC_STRING << 8) | (2*chr_count + 2);
    return _desc_str;
}
