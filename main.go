package main

import (
	"bytes"
	"context"
	_ "embed"
	"github.com/tetratelabs/wazero/api"
	"image"
	"image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/wasi"
)

// drawWasm was built with `tinygo build -o wasm/draw.wasm -scheduler=none -target=wasi wasm/draw.go`
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
	ctx := context.Background()
	wazRuntime := wazero.NewRuntime()
	defer wazRuntime.Close(ctx) // This closes everything this Runtime created.

	// Note: wasm/draw.go doesn't use WASI, but TinyGo needs it to
	// implement functions such as panic.
	if _, err := wasi.InstantiateSnapshotPreview1(ctx, wazRuntime); err != nil {
		log.Panicln(err)
	}

	// Instantiate a WebAssembly module that imports the exports "memory" and
	// the "draw" function which uses Ebiten to render images.
	module, err := wazRuntime.InstantiateModuleFromCode(ctx, drawWasm)
	if err != nil {
		log.Panicln(err)
	}

	for {
		// uses blocking RECEIVE channel to wait until
		// a new circle is requested by the `Update` method
		pos := <-drawNewCircleForPos

		img := draw(ctx, module)

		// transfer through channel the image ready to be used by ebiten
		// with its x and y coordinates already calculated for drawing
		circleReadyToBeDrawn <- circleImage{img: ebiten.NewImageFromImage(img), x: float64(pos%10) * 30, y: float64(((pos / 10) % 10) * 30)}
	}
}

// draw creates a new GO image using https://github.com/fogleman/gg
func draw(ctx context.Context, module api.Module) image.Image {
	// and returns its size in bytes and memory offset packed into a uint64.
	ptrSize, err := module.ExportedFunction("draw").Call(ctx)
	if err != nil {
		log.Panicln(err)
	}

	// Note: This pointer is still owned by TinyGo, so don't try to free it!
	imgPtr := uint32(ptrSize[0] >> 32)
	imgSize := uint32(ptrSize[0])

	// The image represented by region in linear-memory must be read before
	// the next Wasm call. Otherwise, the underlying data could be garbage
	// collected.
	v, ok := module.Memory().Read(ctx, imgPtr, imgSize)
	if !ok {
		log.Panicf("Memory.Read(%d, %d) out of range of memory size %d\n",
			imgPtr, imgSize, module.Memory().Size(ctx))
	}

	// decode bytes and convert them back into a golang image.Image object
	img, err := png.Decode(bytes.NewReader(v))
	if err != nil {
		log.Panicln(err)
	}
	return img
}

// Update is required by Ebiten. It is called
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

// Draw is required by Ebiten. It is called
// automatically and is in charge of rendering to the screen
func (g *Game) Draw(screen *ebiten.Image) {
	op := ebiten.DrawImageOptions{}
	screen.DrawImage(g.img, &op)
}

// Layout is required by Ebiten. It controls rendering proportions.
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
