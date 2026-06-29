package models

import (
	"crypto/rand"
	"fmt"
	"time"
)

// newUUID generates a v4 UUID string without external dependencies.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp + random digits on failure
		return fmt.Sprintf("%016x-%04x-%04x-%04x-%012x",
			uint64(time.Now().UnixNano())>>32,
			uint64(time.Now().UnixNano())>>16&0xffff,
			0x4000|uint64(time.Now().UnixNano())&0xfff,
			0x8000|uint64(time.Now().Nanosecond())&0x3fff,
			uint64(time.Now().UnixNano())&0xffffffffffff)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
