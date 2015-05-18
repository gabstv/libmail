package libmail

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
)

func newBoundary() string {
	bb := make([]byte, 16)
	rand.Read(bb)
	return hex.EncodeToString(bb)
}

type ReadCloserBuffer struct {
	buf *bytes.Buffer
}

func NewReadCloserBuffer(bs []byte) *ReadCloserBuffer {
	out := &ReadCloserBuffer{}
	out.buf = bytes.NewBuffer(bs)
	return out
}

func (r *ReadCloserBuffer) Read(p []byte) (n int, err error) {
	return r.buf.Read(p)
}

func (r *ReadCloserBuffer) Close() error {
	r.buf.Reset()
	return nil
}
