# Golang GOMJPEG package

This is golang mjpeg package that can be used to fetch images from a mjpeg stream in background and convert to golang image. This can be used by applications to render to x11 window or save to local files.


## Sample Usage

For a complete set of examples, please refer to the `example/main.go` file.

```go
package main

import (
	"log"
	"time"

	"github.com/wiless/gomjpeg"
)

func main() {
	// Create a new MJPEG client with options.
	// The URL should be the address of your MJPEG stream.
	// AutoStopTimer will stop the stream after the given seconds of inactivity.
	opts := gomjpeg.MjpegOpts{URL: "http://example.com/mjpeg_stream", AutoStopTimer: 15}
	opts.EnableLog = true // Enable logging for debugging purposes.

	// Tip: You can also configure the client using environment variables.
	// Create a .env file in the same directory as your application with the following content:
	//
	// MJPEG_URL=http://another-example.com/mjpeg_stream
	// MJPEG_AUTOSTOP_TIMER=30
	//
	// The client will automatically load these variables and override the options set in the code.

	mjpegClient := gomjpeg.NewMjpeg(opts)

	// You can optionally get a status channel to monitor the stream's status.
	go func() {
		for _ = range mjpegClient.GetStatusChannel() {
			log.Printf("Stream status: %s", mjpegClient.GetStatusCodeString())
		}
	}()

	// Start the stream. This returns a channel that will receive the images.
	imageChannel := mjpegClient.Start()

	// Read images from the channel in a separate goroutine.
	go func() {
		for img := range imageChannel {
			// Do something with the image, e.g., save it, display it, etc.
			// fmt.Printf("\r[%s] Received image with dimensions: %dx%d", time.Now().Local().Format("15:04:05.00"), img.Bounds().Dx(), img.Bounds().Dy())
		}
	}()

	// Let the stream run for 10 seconds and then stop it.
	log.Println("Running stream for 10 seconds...")
	time.Sleep(10 * time.Second)
	mjpegClient.Stop()
	log.Println("Stream stopped.")

	log.Println("Done.")
}
```

## Configuration
- The MJPEG stream object can be configured using the `MjpegOpts` struct.
- The stream can also be configured using environment variables:
  - `MJPEG_URL`: URL of the MJPEG stream.
  - `MJPEG_AUTOSTOP_TIMER`: Auto-stop timer in seconds (-1 to disable).
  - `MJPEG_RESIZE`: Enable image resizing (e.g., `true`).
  - `MJPEG_WIDTH`: Width of the resized image.
  - `MJPEG_HEIGHT`: Height of the resized image.
  - `MJPEG_ENABLE_LOG`: Enable or disable logging (e.g., `true`).

## Concept
- The library uses the `godotenv` package to load environment variables from a `.env` file.
