package main

import (
	"bytes"
	"github.com/fogleman/gg"
	"image/png"
	"math/rand"
	"time"
	"unsafe"
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func draw() ([]byte, error) {
	dc := gg.NewContext(30, 30)
	dc.DrawCircle(15, 15, 15)
	dc.SetRGB(rand.Float64(), rand.Float64(), rand.Float64())
	dc.Fill()
	img := dc.Image()

	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// _draw is a WebAssembly export that invokes draw and returns a pointer/size
// pair of the image, packed into a uint64.
//
// Note: This uses a uint64 instead of two result values for compatibility
// with WebAssembly 1.0.
// See https://github.com/tetratelabs/wazero/tree/main/examples/allocation/tinygo
//export draw
func _draw() (ptrSize uint64) {
	buf, err := draw()
	if err != nil {
		panic(err)
	}
	ptr, size := bufToPtr(buf)
	return (uint64(ptr) << uint64(32)) | uint64(size)
}

// bufToPtr returns a pointer and size pair for the given byte slice in a way
// compatible with WebAssembly numeric types.
func bufToPtr(buf []byte) (uint32, uint32) {
	ptr := &buf[0]
	unsafePtr := uintptr(unsafe.Pointer(ptr))
	return uint32(unsafePtr), uint32(len(buf))
}
