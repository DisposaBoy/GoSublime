#ifndef CBOR_H
#define CBOR_H

#define CBOR_TYPE_MASK 0xE0  /* top 3 bits */
#define CBOR_INFO_BITS 0x1F  /* low 5 bits */

#define CBOR_UINT      0x00
#define CBOR_NEGINT    0x20
#define CBOR_BYTES     0x40
#define CBOR_TEXT      0x60
#define CBOR_ARRAY     0x80
#define CBOR_MAP       0xA0
#define CBOR_TAG       0xC0
#define CBOR_7         0xE0  /* float and other types */

#define CBOR_ADDITIONAL_INFORMATION 0x1F

/* read the "additional information" of a tag byte which is often a
 * small literal integer describing the length in bytes of the data
 * item */
#define IS_SMALL_LITERAL(n) (((n) & 0x1f) < 24)
#define SMALL_LITERAL(n) ((n) & 0x1f)


#define CBOR_UINT8_FOLLOWS   24 // 0x18
#define CBOR_UINT16_FOLLOWS  25 // 0x19
#define CBOR_UINT32_FOLLOWS  26 // 0x1A
#define CBOR_UINT64_FOLLOWS  27 // 0x1B
#define CBOR_VAR_FOLLOWS     31 // 0x1F

#define CBOR_UINT8   (CBOR_UINT | CBOR_UINT8_FOLLOWS)
#define CBOR_UINT16  (CBOR_UINT | CBOR_UINT16_FOLLOWS)
#define CBOR_UINT32  (CBOR_UINT | CBOR_UINT32_FOLLOWS)
#define CBOR_UINT64  (CBOR_UINT | CBOR_UINT64_FOLLOWS)

#define CBOR_NEGINT8   (CBOR_NEGINT | CBOR_UINT8_FOLLOWS)
#define CBOR_NEGINT16  (CBOR_NEGINT | CBOR_UINT16_FOLLOWS)
#define CBOR_NEGINT32  (CBOR_NEGINT | CBOR_UINT32_FOLLOWS)
#define CBOR_NEGINT64  (CBOR_NEGINT | CBOR_UINT64_FOLLOWS)


#define CBOR_BREAK 0xFF

#define CBOR_FALSE   (CBOR_7 | 20)
#define CBOR_TRUE    (CBOR_7 | 21)
#define CBOR_NULL    (CBOR_7 | 22)
#define CBOR_UNDEFINED (CBOR_7 | 23)

#define CBOR_FLOAT16 (CBOR_7 | 25)
#define CBOR_FLOAT32 (CBOR_7 | 26)
#define CBOR_FLOAT64 (CBOR_7 | 27)


#define CBOR_TAG_DATE_STRING (0) /* RFC3339 */
#define CBOR_TAG_DATE_ARRAY (1) /* any number type follows, seconds since 1970-01-01T00:00:00 UTC */
#define CBOR_TAG_BIGNUM (2)  /* big endian byte string follows */
#define CBOR_TAG_NEGBIGNUM (3)  /* big endian byte string follows */
#define CBOR_TAG_DECIMAL (4) /* [ 10^x exponent, number ] */
#define CBOR_TAG_BIGFLOAT (5) /* [ 2^x exponent, number ] */
//#define CBOR_TAG_BASE64URL (21)
//#define CBOR_TAG_BASE64 (22)
#define CBOR_TAG_BASE16 (23)
#define CBOR_TAG_CBOR (24) /* following byte string is embedded CBOR data */

#define CBOR_TAG_URI 32
//#define CBOR_TAG_BASE64URL 33
//#define CBOR_TAG_BASE64 34
#define CBOR_TAG_REGEX 35
#define CBOR_TAG_MIME 36 /* following text is MIME message, headers, separators and all */
#define CBOR_TAG_CBOR_FILEHEADER 55799  /* can open a file with 0xd9d9f7 */


/* Content-Type: application/cbor */


#endif /* CBOR_H */
