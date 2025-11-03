// Package gomjpeg provides a client for fetching and decoding MJPEG streams.
package gomjpeg

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// StatusCode represents the status of the MJPEG stream.
type StatusCode int

// StreamControl represents a command to control the MJPEG stream.
type StreamControl int

const (
	StatusPlaying StatusCode = iota
	StatusStopped
	StatusError
	StatusPaused
)

const (
	StartStream StreamControl = iota
	StopStream
	PauseStream
	ResumeStream
)

// MjpegOpts holds configuration options for the MJPEG stream.
type MjpegOpts struct {
	// URL of the MJPEG stream.
	URL string
	// AutoStopTimer stops the stream after the specified duration in seconds.
	// A value of -1 disables the auto-stop timer.
	AutoStopTimer int
	// Resize enables resizing of the received JPEG images.
	Resize bool
	// Width of the resized image.
	Width int
	// Height of the resized image.
	Height int
	// EnableLog enables or disables logging.
	EnableLog bool
}

// Mjpeg represents an MJPEG stream client.
type Mjpeg struct {
	opts           MjpegOpts
	client         http.Client
	buffer         bytes.Buffer
	mu             sync.Mutex
	statusChannel  chan StatusCode
	controlChannel chan StreamControl
	statusCode     StatusCode
	// ImageStream is a channel that receives decoded images from the stream.
	ImageStream  chan image.Image
	internalCH   chan StreamControl
	stopDecodeCh chan struct{}
	wg           sync.WaitGroup
	// EnableLog enables or disables logging.
	EnableLog bool
	timer     *time.Timer
}

// loadEnvOverrides loads environment variables from a .env file and overrides the MjpegOpts fields if the corresponding environment variables are set.
func loadEnvOverrides(opts *MjpegOpts) {
	if url := os.Getenv("MJPEG_URL"); url != "" {
		opts.URL = url
	}
	if autoStopTimer := os.Getenv("MJPEG_AUTOSTOP_TIMER"); autoStopTimer != "" {
		if val, err := strconv.Atoi(autoStopTimer); err == nil {
			opts.AutoStopTimer = val
		}
	}
	if resize := os.Getenv("MJPEG_RESIZE"); resize != "" {
		if val, err := strconv.ParseBool(resize); err == nil {
			opts.Resize = val
		}
	}
	if width := os.Getenv("MJPEG_WIDTH"); width != "" {
		if val, err := strconv.Atoi(width); err == nil {
			opts.Width = val
		}
	}
	if height := os.Getenv("MJPEG_HEIGHT"); height != "" {
		if val, err := strconv.Atoi(height); err == nil {
			opts.Height = val
		}
	}
	if enableLog := os.Getenv("MJPEG_ENABLE_LOG"); enableLog != "" {
		if val, err := strconv.ParseBool(enableLog); err == nil {
			opts.EnableLog = val
		}
	}
}

// NewMjpeg creates a new Mjpeg client with the given options.
// It loads environment variables from a .env file to override the options.
func NewMjpeg(opts MjpegOpts) *Mjpeg {
	godotenv.Load()
	loadEnvOverrides(&opts)

	m := &Mjpeg{
		opts:           opts,
		controlChannel: make(chan StreamControl),
		internalCH:     make(chan StreamControl),
		stopDecodeCh:   make(chan struct{}),
		EnableLog:      opts.EnableLog,
	}
	m.init()
	return m
}

func (m *Mjpeg) logf(format string, v ...interface{}) {
	if m.EnableLog {
		log.Printf(format, v...)
	}
}

func (m *Mjpeg) init() {
	m.statusChannel = make(chan StatusCode, 1)
	m.setStatusCode(StatusStopped)
}

// GetControlChannel returns the channel for controlling the stream.
func (m *Mjpeg) GetControlChannel() chan StreamControl {
	return m.controlChannel
}

// GetStatusChannel returns the channel for receiving status updates.
func (m *Mjpeg) GetStatusChannel() chan StatusCode {
	return m.statusChannel
}

// GetStatusCode returns the current status code of the stream.
func (m *Mjpeg) GetStatusCode() StatusCode {
	return m.statusCode
}

// GetStatusCodeString returns the current status as a string.
func (m *Mjpeg) GetStatusCodeString() string {
	switch m.statusCode {
	case StatusPlaying:
		return "Playing"
	case StatusStopped:
		return "Stopped"
	case StatusError:
		return "Error"
	case StatusPaused:
		return "Paused"
	default:
		return "Unknown"
	}
}

func (m *Mjpeg) setStatusCode(statusCode StatusCode) {
	m.statusCode = statusCode
	select {
	case m.statusChannel <- m.statusCode:
	default:
	}
}

// Start begins fetching images from the MJPEG stream in a goroutine.
// It returns a channel that receives the decoded images.
func (m *Mjpeg) Start() chan image.Image {
	if m.ImageStream != nil {
		close(m.ImageStream)
	}
	m.ImageStream = make(chan image.Image, 1) // Buffered channel
	go m.startFetching()
	m.internalCH <- StartStream

	return m.ImageStream
}

// Stop stops the MJPEG stream.
func (m *Mjpeg) Stop() {
	m.internalCH <- StopStream
}

// Pause pauses the MJPEG stream.
func (m *Mjpeg) Pause() {
	m.internalCH <- PauseStream

}

// Resume resumes the MJPEG stream.
func (m *Mjpeg) Resume() {
	m.internalCH <- ResumeStream
}

// ResetTimer resets the auto-stop timer with a new duration.
func (m *Mjpeg) ResetTimer(duration int) {
	if m.timer != nil {
		m.timer.Stop()
	}
	m.logf("\nResetting AutoStopTimer for %d seconds.", duration)

	if duration > 0 {
		m.opts.AutoStopTimer = duration

	}
	m.timer = time.AfterFunc(time.Duration(m.opts.AutoStopTimer)*time.Second, func() {
		m.Stop()
	})
}

// getStreamResponse handles making the HTTP GET request and returns the response.
func (m *Mjpeg) getStreamResponse() (*http.Response, error) {
	res, err := http.Get(m.opts.URL)
	if err != nil {
		return nil, fmt.Errorf("error getting response from server: %w", err)
	}
	m.logf("Got response from server: %s", res.Status)
	return res, nil
}

// parseContentTypeAndBoundary parses the Content-Type header to extract the boundary string.
func (m *Mjpeg) parseContentTypeAndBoundary(contentType string) (string, error) {
	m.logf("Content-Type: %v", contentType)
	boundary := ""
	if strings.HasPrefix(contentType, "multipart/x-mixed-replace") {
		parts := strings.Split(contentType, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "boundary=") {
				boundary = "--" + strings.TrimPrefix(part, "boundary=")
				break
			}
		}
	}

	if boundary == "" {
		m.logf("Error: Could not find boundary in Content-Type header. Assuming default '--frame'")
		boundary = "--frame"
	}
	return boundary, nil
}

func (m *Mjpeg) startFetching() {
	var reader *bufio.Reader
	var res *http.Response

	defer func() {
		if res != nil {
			res.Body.Close()
		}
		close(m.ImageStream)
		m.setStatusCode(StatusStopped)
	}()

	for {
		select {
		case control := <-m.internalCH:
			switch control {
			case StartStream:
				if m.GetStatusCode() == StatusPlaying {
					continue
				}
				m.setStatusCode(StatusPlaying)

				var err error

				res, err = m.getStreamResponse()
				if err != nil {
					m.logf("Error getting response: %v", err)
					m.setStatusCode(StatusError)
					return
				}

				contentType := res.Header.Get("Content-Type")
				boundary, err := m.parseContentTypeAndBoundary(contentType)
				if err != nil {
					m.logf("Error parsing content type and boundary: %v", err)
					m.setStatusCode(StatusError)
					return
				}

				reader = bufio.NewReader(res.Body)
				m.wg.Add(1)
				go m.decodeStream(reader, boundary)

			case StopStream:
				close(m.stopDecodeCh)
				m.wg.Wait()
				return

			case PauseStream:
				m.setStatusCode(StatusPaused)

			case ResumeStream:
				m.setStatusCode(StatusPlaying)
			}
		}
	}
}

// readImageHeaders reads headers for a JPEG image part and returns its Content-Length.
func (m *Mjpeg) readImageHeaders(reader *bufio.Reader) (int, error) {
	var contentLength int
	for {
		headerLine, err := reader.ReadString('\n')
		if err != nil {
			return 0, fmt.Errorf("error reading header: %w", err)
		}

		if strings.TrimSpace(headerLine) == "" {
			break // End of headers
		}

		if strings.HasPrefix(headerLine, "Content-Length:") {
			fmt.Sscanf(headerLine, "Content-Length: %d", &contentLength)
		}
	}
	return contentLength, nil
}

// processAndSendImage decodes JPEG data and sends it to the ImageStream.
// It also manages the auto-stop timer.
func (m *Mjpeg) processAndSendImage(jpegData []byte, timerStarted *bool) {
	if len(jpegData) == 0 {
		return
	}

	img, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		m.logf("Error decoding JPEG: %v", err)
		return
	}

	// TODO: Implement image resizing if m.opts.Resize is true

	// Non-blocking write to ImageStream
	select {
	case m.ImageStream <- img:
		if !*timerStarted && m.opts.AutoStopTimer > 0 {
			m.logf("\nFirst image received. Starting AutoStopTimer for %d seconds.", m.opts.AutoStopTimer)
			m.timer = time.AfterFunc(time.Duration(m.opts.AutoStopTimer)*time.Second, func() {
				m.Stop()
			})
			*timerStarted = true
		}
	default:
		// ImageStream is full, drop the image
	}
}

func (m *Mjpeg) decodeStream(reader *bufio.Reader, boundary string) {
	defer m.wg.Done()
	imgcounter := 0 // This variable is not used after refactoring, can be removed later if not needed.
	timerStarted := false
	m.logf("Starting decodeStream goroutine")

	for {
		select {
		case <-m.stopDecodeCh:
			m.logf("Stopping decodeStream goroutine")
			return
		case control := <-m.internalCH:
			switch control {
			case PauseStream:
				m.setStatusCode(StatusPaused)
				continue
			case ResumeStream:
				m.setStatusCode(StatusPlaying)
				continue
			}
		default:
			if m.statusCode == StatusPaused {
				time.Sleep(100 * time.Millisecond) // Prevent busy-waiting
				continue
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				m.logf("Error reading line: %v", err)
				m.setStatusCode(StatusError)
				break
			}

			if strings.Contains(line, boundary) {
				contentLength, err := m.readImageHeaders(reader)
				if err != nil {
					m.logf("Error reading image headers: %v", err)
					m.setStatusCode(StatusError)
					continue
				}

			jpegData := make([]byte, contentLength)
			_, err = io.ReadFull(reader, jpegData)
			if err != nil {
				m.logf("Error reading JPEG data: %v", err)
				m.setStatusCode(StatusError)
				continue
			}

			m.processAndSendImage(jpegData, &timerStarted)
			}
		}
	}
}