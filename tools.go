package libmail

import (
	"crypto/rand"
	"encoding/hex"
)

func newBoundary() string {
	bb := make([]byte, 16)
	rand.Read(bb)
	return hex.EncodeToString(bb)
}
