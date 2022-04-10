package wasm

import (
	"bytes"
	"github.com/fogleman/gg"
	"image/png"
	"math/rand"
	"time"
)

const BUFFER_SIZE = 1000

var imageBytes [BUFFER_SIZE]byte
var imageSize int
var buf *bytes.Buffer

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
	buf = new(bytes.Buffer)
}

//export draw
func Draw() {
	dc := gg.NewContext(30, 30)
	dc.DrawCircle(15, 15, 15)
	dc.SetRGB(rand.Float64(), rand.Float64(), rand.Float64())
	dc.Fill()
	img := dc.Image()

	err := png.Encode(buf, img)
	if err != nil {
		panic(err)
	}
	copy(imageBytes[:], buf.Bytes())
	buf.Reset()
}

//export getImageAddress
func GetImageAddress() *[BUFFER_SIZE]uint8 {
	return &imageBytes
}

//export getImageSize
func GetImageSize() int {
	return len(imageBytes)
}
