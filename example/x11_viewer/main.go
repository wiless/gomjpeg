package main

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/xgb" // X11 library
	"github.com/BurntSushi/xgb/xproto"

	"github.com/wiless/gomjpeg"
)

const (
	WIDTH  = 800
	HEIGHT = 600
)

// putImage draws an image.Image to the X11 window.
func putImage(X *xgb.Conn, win xproto.Window, gc xproto.Gcontext, img image.Image) {
	rgba, ok := img.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	}

	// Create an XImage from the RGBA pixel data.
	// This assumes a 24-bit TrueColor visual (common).
	// The depth and format might need to be adjusted for different X11 setups.
	xproto.PutImage(
		X.Conn(),
		xproto.ImageFormatZpixmap, // ZPixmap is common for TrueColor visuals
		win,
		gc,
		uint16(rgba.Rect.Dx()),
		uint16(rgba.Rect.Dy()),
		0, 0, // dest_x, dest_y
		24,   // depth (assuming 24-bit color)
		rgba.Pix,
	)
	xproto.ImageText8(X.Conn(), byte(len("MJPEG Viewer")), win, gc, 10, 20, []byte("MJPEG Viewer"))
	xproto.ImageText8(X.Conn(), byte(len(fmt.Sprintf("Image: %dx%d", rgba.Rect.Dx(), rgba.Rect.Dy()))), win, gc, 10, 40, []byte(fmt.Sprintf("Image: %dx%d", rgba.Rect.Dx(), rgba.Rect.Dy())))
}

func main() {
	// 1. Initialize MJPEG client
	opts := gomjpeg.MjpegOpts{URL: "http://example.com/mjpeg_stream", AutoStopTimer: -1} // -1 to keep stream alive
	opts.EnableLog = true
	mjpegClient := gomjpeg.NewMjpeg(opts)

	imageChannel := mjpegClient.Start()

	// 2. Initialize X11 connection
	X, err := xgb.NewConn()
	if err != nil {
		log.Fatalf("Failed to connect to X server: %v", err)
	}
	defer X.Close()

	setup := xproto.Setup(X)
	screen := setup.DefaultScreen(X)

	// 3. Create X11 window
	win, err := xproto.NewWindowId(X.Conn())
	if err != nil {
		log.Fatalf("Failed to generate new window ID: %v", err)
	}

	mask := xproto.CwBackPixel | xproto.CwEventMask
	values := []uint32{
		screen.WhitePixel,
		xproto.EventMaskExposure | xproto.EventMaskKeyPress | xproto.EventMaskStructureNotify,
	}

	xproto.CreateWindow(
		X.Conn(),
		screen.RootDepth,
		win,
		screen.Root,
		0, 0, // x, y
		WIDTH, HEIGHT, // width, height
		0,             // border_width
		xproto.WindowClassInputOutput,
		screen.RootVisual,
		mask,
		values,
	)

	xproto.MapWindow(X.Conn(), win)
	xproto.ChangeProperty(
		X.Conn(),
		xproto.PropModeReplace,
		win,
		xproto.AtomWmName,
		xproto.AtomString,
		8,
		uint32(len("MJPEG Viewer")), []byte("MJPEG Viewer"),
	)

	// 4. Create Graphics Context (GC)
	gc, err := xproto.NewGcontextId(X.Conn())
	if err != nil {
		log.Fatalf("Failed to generate new GC ID: %v", err)
	}
	xproto.CreateGc(X.Conn(), gc, win, 0, []uint32{})

	// 5. Image rendering loop
	go func() {
		for img := range imageChannel {
			putImage(X, win, gc, img)
		}
	}()

	// 6. X11 Event loop
	for {
		event, err := X.WaitForEvent()
		if err != nil {
			log.Printf("X11 Event error: %v", err)
			continue
		}

		switch e := event.(type) {
		case xproto.ExposeEvent:
			// Window needs redrawing. In a real app, you'd redraw the last received image.
			log.Println("Expose event - window needs redraw.")
		case xproto.KeyPressEvent:
			// Handle key presses, e.g., 'q' to quit
			if e.Detail == 24 { // 'q' keycode
				log.Println("'q' pressed. Exiting.")
				return
			}
		case xproto.ClientMessageEvent:
			// Handle window close button
			// This requires more complex setup to intercept WM_DELETE_WINDOW
			log.Println("ClientMessageEvent - potential window close.")
		case xgb.Error: // Handle X11 errors
			log.Printf("X11 Error: %v", e)
		}
	}
}