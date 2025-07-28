package wav

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/rtc"
)

// Header represents a WAV file header
type Header struct {
	ChunkSize    uint32
	SampleRate   uint32
	NumChannels  uint16
	BitsPerSample uint16
	DataSize     uint32
}

// Reader reads WAV files and converts them to AudioFrames
type Reader struct {
	file   *os.File
	header Header
}

// NewReader creates a new WAV file reader
func NewReader(filename string) (*Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAV file: %w", err)
	}

	reader := &Reader{file: file}
	if err := reader.readHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read WAV header: %w", err)
	}

	return reader, nil
}

// Header returns the WAV file header information
func (r *Reader) Header() Header {
	return r.header
}

// ReadFrames reads the entire WAV file and returns it as 10ms AudioFrames
func (r *Reader) ReadFrames() ([]rtc.AudioFrame, error) {
	// Calculate frame size for 10ms
	samplesPerFrame := int(r.header.SampleRate) / 100 // 10ms worth of samples
	bytesPerFrame := samplesPerFrame * int(r.header.NumChannels) * (int(r.header.BitsPerSample) / 8)

	var frames []rtc.AudioFrame
	buffer := make([]byte, bytesPerFrame)
	frameIndex := 0

	for {
		n, err := r.file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read audio data: %w", err)
		}

		// Pad with zeros if we didn't read a full frame
		if n < bytesPerFrame {
			for i := n; i < bytesPerFrame; i++ {
				buffer[i] = 0
			}
		}

		frame := rtc.AudioFrame{
			Data:              make([]byte, bytesPerFrame),
			SampleRate:        int(r.header.SampleRate),
			SamplesPerChannel: samplesPerFrame,
			NumChannels:       int(r.header.NumChannels),
			Timestamp:         time.Duration(frameIndex) * 10 * time.Millisecond,
		}

		copy(frame.Data, buffer[:bytesPerFrame])
		frames = append(frames, frame)
		frameIndex++
	}

	return frames, nil
}

// Close closes the WAV file
func (r *Reader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// readHeader reads and validates the WAV file header
func (r *Reader) readHeader() error {
	// Read RIFF header
	var riffHeader [12]byte
	if _, err := r.file.Read(riffHeader[:]); err != nil {
		return fmt.Errorf("failed to read RIFF header: %w", err)
	}

	// Validate RIFF signature
	if string(riffHeader[0:4]) != "RIFF" {
		return fmt.Errorf("not a valid RIFF file")
	}

	// Validate WAVE signature
	if string(riffHeader[8:12]) != "WAVE" {
		return fmt.Errorf("not a valid WAVE file")
	}

	r.header.ChunkSize = binary.LittleEndian.Uint32(riffHeader[4:8])

	// Find and read fmt chunk
	if err := r.readFmtChunk(); err != nil {
		return err
	}

	// Find and read data chunk
	if err := r.readDataChunk(); err != nil {
		return err
	}

	// Validate format
	if r.header.BitsPerSample != 16 {
		return fmt.Errorf("only 16-bit samples are supported, got %d-bit", r.header.BitsPerSample)
	}

	if r.header.NumChannels != 1 && r.header.NumChannels != 2 {
		return fmt.Errorf("only mono and stereo are supported, got %d channels", r.header.NumChannels)
	}

	if r.header.SampleRate != 16000 && r.header.SampleRate != 48000 {
		return fmt.Errorf("only 16kHz and 48kHz sample rates are supported, got %dHz", r.header.SampleRate)
	}

	return nil
}

// readFmtChunk reads the format chunk
func (r *Reader) readFmtChunk() error {
	for {
		var chunkHeader [8]byte
		if _, err := r.file.Read(chunkHeader[:]); err != nil {
			return fmt.Errorf("failed to read chunk header: %w", err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		if chunkID == "fmt " {
			if chunkSize < 16 {
				return fmt.Errorf("fmt chunk too small: %d bytes", chunkSize)
			}

			var fmtData [16]byte
			if _, err := r.file.Read(fmtData[:]); err != nil {
				return fmt.Errorf("failed to read fmt data: %w", err)
			}

			audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
			if audioFormat != 1 {
				return fmt.Errorf("only PCM format is supported, got format %d", audioFormat)
			}

			r.header.NumChannels = binary.LittleEndian.Uint16(fmtData[2:4])
			r.header.SampleRate = binary.LittleEndian.Uint32(fmtData[4:8])
			r.header.BitsPerSample = binary.LittleEndian.Uint16(fmtData[14:16])

			// Skip any remaining fmt data
			if chunkSize > 16 {
				if _, err := r.file.Seek(int64(chunkSize-16), io.SeekCurrent); err != nil {
					return fmt.Errorf("failed to skip fmt data: %w", err)
				}
			}

			return nil
		}

		// Skip unknown chunk
		if _, err := r.file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
			return fmt.Errorf("failed to skip chunk: %w", err)
		}
	}
}

// readDataChunk finds the data chunk and positions the file pointer at the start of audio data
func (r *Reader) readDataChunk() error {
	for {
		var chunkHeader [8]byte
		if _, err := r.file.Read(chunkHeader[:]); err != nil {
			return fmt.Errorf("failed to read chunk header: %w", err)
		}

		chunkID := string(chunkHeader[0:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:8])

		if chunkID == "data" {
			r.header.DataSize = chunkSize
			// File pointer is now at the start of audio data
			return nil
		}

		// Skip unknown chunk
		if _, err := r.file.Seek(int64(chunkSize), io.SeekCurrent); err != nil {
			return fmt.Errorf("failed to skip chunk: %w", err)
		}
	}
}