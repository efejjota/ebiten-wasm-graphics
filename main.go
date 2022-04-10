package main

import (
	"bytes"
	_ "embed"
	"image/png"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/wasi"
)

//go:embed wasm/draw.wasm
var drawWasm []byte

type Game struct {
	pos int
	img *ebiten.Image
}

type circleImage struct {
	x, y float64
	img  *ebiten.Image
}

var circleReadyToBeDrawn chan circleImage
var drawNewCircleForPos chan int

func drawWasmCircle() {
	wazRuntime := wazero.NewRuntime()
	wm, _ := wasi.InstantiateSnapshotPreview1(wazRuntime)
	module, _ := wazRuntime.InstantiateModuleFromCodeWithConfig(drawWasm, wazero.NewModuleConfig())
	defer wm.Close()
	defer module.Close()

	draw := module.ExportedFunction("draw")
	getImageAddress := module.ExportedFunction("getImageAddress")
	getImageSize := module.ExportedFunction("getImageSize")
	memory := module.Memory()

	for {
		// uses blocking RECEIVE channel to wait until
		// a new circle is requested by the `Update` method
		pos := <-drawNewCircleForPos

		// draw function creates a new GO image using https://github.com/fogleman/gg
		// and stores it in a fixed size []byte slice called `imageBytes`
		draw.Call(nil)

		// getImageSize returns the number of bytes that correspond to the generated image
		imgSize, _ := getImageSize.Call(nil)

		// getImageAddress returns a pointer to `imageBytes` which we need to use
		// as the offset within wasm linear-memory in the following instruction
		imgAddr, _ := getImageAddress.Call(nil)

		// read from linear-memory the image as a slice of bytes encoded in png format
		v, _ := memory.Read(uint32(imgAddr[0]), uint32(imgSize[0]))

		// decode bytes and convert them back into a golang image.Image object
		img, _ := png.Decode(bytes.NewReader(v))

		// transfer through channel the image ready to be used by ebiten
		// with its x and y coordinates already calculated for drawing
		circleReadyToBeDrawn <- circleImage{img: ebiten.NewImageFromImage(img), x: float64(pos%10) * 30, y: float64(((pos / 10) % 10) * 30)}
	}
}

// This method is required by Ebiten. It is called
// automatically and includes the logic communicating
// with the `drawWasmCircle` function wich runs Wazero
func (g *Game) Update() error {
	select {
	case newCircle := <-circleReadyToBeDrawn:
		op := ebiten.DrawImageOptions{}
		op.GeoM.Translate(newCircle.x, newCircle.y)
		g.img.DrawImage(newCircle.img, &op)

	default:
		// Don't block if there isn't a new `circleReadyToBeDrawn`
	}

	select {
	// uses non-blocking SEND channel to start drawing a new circle
	// only if the channel is not full (currently configured to 1)
	case drawNewCircleForPos <- g.pos:
		g.pos += 1
	default:
		// Don't block if `drawWasmCircle` has not finished drawing
	}
	return nil
}

// This method is required by Ebiten. It is called
// automatically and is in charge of rendering to the screen
func (g *Game) Draw(screen *ebiten.Image) {
	op := ebiten.DrawImageOptions{}
	screen.DrawImage(g.img, &op)
}

// This method is required by Ebiten. It controls rendering proportions.
// This code is mostly irrelevant for this example.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

func main() {
	ebiten.SetWindowSize(300, 300)
	img := ebiten.NewImage(300, 300)

	// we use these channels to pass data between the
	// `drawWasmCircle` and the `Update` method
	circleReadyToBeDrawn = make(chan circleImage, 1)
	drawNewCircleForPos = make(chan int, 1)

	// in the following goroutine we initialize Wazero.
	// the goroutine creates an infinite loop which waits
	// for new messages coming from `drawNewCircleForPos` channel.

	// Wazero is used to call a WASM function which
	// creates an image of a circle with a random color
	// this image is then sent to the `Update` method
	// through the `circleReadyToBeDrawn` channel
	// and is then rendered to the screen by ebiten.
	go drawWasmCircle()

	// the Update method is called periodically by ebiten.RunGame
	// the following instruction blocks until the Window is closed
	if err := ebiten.RunGame(&Game{img: img, pos: 0}); err != nil {
		panic(err)
	}
}
