package utils

import (
	"unsafe"
)

func Bytes2string(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}
