package main

import (
	"fmt"
	"log"
	"time"

	"github.com/wiless/gomjpeg"
)

func main() {
	// Tip: You can also configure the client using environment variables.
	// Create a .env file in the same directory as your application with the following content:
	//
	// MJPEG_URL=http://another-example.com/mjpeg_stream
	// MJPEG_AUTOSTOP_TIMER=30
	//
	// The client will automatically load these variables and override the options set in the code.

	// Call one of the example functions below.
	// Comment out the others to run a specific example.
	// Example1()
	// Example2()
	Example3()

	log.Println("Done with main.")
}

// Example1 demonstrates how to start an MJPEG stream and then stop it after a duration.
func Example1() {
	log.Println("---" + " Example 1: Running stream for 10/Manual Stop seconds...")
	// Create a new MJPEG client for this example.
	opts1 := gomjpeg.MjpegOpts{URL: "http://example.com/mjpeg_stream"}
	opts1.EnableLog = true // Enable logging for debugging purposes.
	mjpegClient1 := gomjpeg.NewMjpeg(opts1)

	// You can optionally get a status channel to monitor the stream's status.
	go func() {
		for _ = range mjpegClient1.GetStatusChannel() {
			log.Printf("Stream status (Example 1): %s", mjpegClient1.GetStatusCodeString())
		}
	}()

	// Start the stream. This returns a channel that will receive the images.
	imageChannel1 := mjpegClient1.Start()

	// Read images from the channel in a separate goroutine.
	go func() {
		for img := range imageChannel1 {
			fmt.Printf("\r[Example 1 %s]  Received image with dimensions: %dx%d", time.Now().Local().Format("03:04:05.00"), img.Bounds().Dx(), img.Bounds().Dy())
		}
	}()

	time.Sleep(10 * time.Second)
	mjpegClient1.Stop()
	log.Println("Stream stopped (Example 1).")

}

// Example2 demonstrates how to manually pause and resume an MJPEG stream.
func Example2() {
	log.Println("---" + " Example 2: Starting stream for manual pause/resume example...")
	// Create a new MJPEG client for this example.
	opts2 := gomjpeg.MjpegOpts{URL: "http://example.com/mjpeg_stream"}
	opts2.EnableLog = true // Enable logging for debugging purposes.
	mjpegClient2 := gomjpeg.NewMjpeg(opts2)

	go func() {
		for _ = range mjpegClient2.GetStatusChannel() {
			log.Printf("Stream status (Example 2): %s", mjpegClient2.GetStatusCodeString())
		}
	}()

	imageChannel2 := mjpegClient2.Start()

	go func() {
		for img := range imageChannel2 {
			fmt.Printf("\r[Example 2 %s] Received image with dimensions: %dx%d", time.Now().Local().Format("03:04:05.00"), img.Bounds().Dx(), img.Bounds().Dy())
		}
	}()

	time.Sleep(5 * time.Second)
	log.Println("Pausing stream (Example 2)...")
	mjpegClient2.Pause()
	time.Sleep(5 * time.Second)
	log.Println("Resuming stream (Example 2)...")
	mjpegClient2.Resume()
	time.Sleep(5 * time.Second)
	mjpegClient2.Stop()
	log.Println("Stream stopped (Example 2).")

}

// Example3 demonstrates using the AutoStopTimer and how to reset it.
func Example3() {
	log.Println("---" + " Example 3: Starting stream with AutoStopTimer (15s)...")
	// Create a new MJPEG client for this example.
	opts3 := gomjpeg.MjpegOpts{URL: "http://example.com/mjpeg_stream", AutoStopTimer: 15}
	opts3.EnableLog = true // Enable logging for debugging purposes.
	mjpegClient3 := gomjpeg.NewMjpeg(opts3)

	go func() {
		for _ = range mjpegClient3.GetStatusChannel() {
			log.Printf("Stream status (Example 3): %s", mjpegClient3.GetStatusCodeString())
		}
	}()

	imageChannel3 := mjpegClient3.Start()

	go func() {
		for img := range imageChannel3 {
			fmt.Printf("\r[Example 3 %s] Received image with dimensions: %dx%d", time.Now().Local().Format("03:04:05.00"), img.Bounds().Dx(), img.Bounds().Dy())
		}
	}()

	log.Println("Sleeping for 10 seconds (Example 3)...")
	time.Sleep(10 * time.Second)
	log.Println("Resetting timer to 20 seconds (Example 3)...")
	mjpegClient3.ResetTimer(20) // Resets the timer to 20 seconds.
	log.Println("Sleeping for another 25 seconds (Example 3)...")
	time.Sleep(25 * time.Second)
	// log.Println("The stream should still be running. It will stop after another 2 seconds of inactivity (Example 3).")
	// time.Sleep(2 * time.Second) // Wait for the stream to auto-stop
	log.Println("Stream should be stopped now (Example 3).")
}
