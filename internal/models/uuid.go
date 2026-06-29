package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// newUUID generates a v4 UUID string (RFC 9562).
// Falls back to crypto/rand hex if full v4 generation fails.
func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Degraded fallback: 32 random hex chars formatted as UUID
		fallback := make([]byte, 20)
		rand.Read(fallback)
		h := hex.EncodeToString(fallback)
		return fmt.Sprintf("%s-%s-%s-%s-%s",
			h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10xx
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
