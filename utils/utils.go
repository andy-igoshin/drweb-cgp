package utils

import (
	"unsafe"
)

func Bytes2string(b []byte) string {
	// WARNING: unsafe.String shares the backing memory with b.
	// The caller must ensure the returned string is not used after
	// the original []byte is freed or mutated.
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}
