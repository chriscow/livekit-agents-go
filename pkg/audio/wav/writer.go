package wav

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

// Writer writes WAV files
type Writer struct {
	file         *os.File
	sampleRate   uint32
	numChannels  uint16
	bitsPerSample uint16
	samplesWritten uint32
}

// NewWriter creates a new WAV file writer
func NewWriter(filename string, sampleRate uint32, numChannels, bitsPerSample uint16) (*Writer, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAV file: %w", err)
	}

	writer := &Writer{
		file:          file,
		sampleRate:    sampleRate,
		numChannels:   numChannels,
		bitsPerSample: bitsPerSample,
	}

	// Write header (we'll update it when we close)
	if err := writer.writeHeader(); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write WAV header: %w", err)
	}

	return writer, nil
}

// WriteSineWave writes a sine wave of the specified frequency and duration
func (w *Writer) WriteSineWave(frequency float64, durationMs int) error {
	samplesPerChannel := int(w.sampleRate) * durationMs / 1000

	for i := 0; i < samplesPerChannel; i++ {
		// Generate sine wave sample
		t := float64(i) / float64(w.sampleRate)
		sample := math.Sin(2 * math.Pi * frequency * t)
		
		// Convert to 16-bit signed integer
		intSample := int16(sample * 32767 * 0.5) // 50% amplitude

		// Write sample for each channel
		for ch := 0; ch < int(w.numChannels); ch++ {
			if err := binary.Write(w.file, binary.LittleEndian, intSample); err != nil {
				return fmt.Errorf("failed to write sample: %w", err)
			}
		}
		
		w.samplesWritten++
	}

	return nil
}

// Close finalizes the WAV file by updating the header with correct sizes
func (w *Writer) Close() error {
	if w.file == nil {
		return nil
	}

	// Update header with actual sizes
	dataSize := w.samplesWritten * uint32(w.numChannels) * uint32(w.bitsPerSample) / 8
	chunkSize := dataSize + 36

	// Seek to chunk size position and update
	if _, err := w.file.Seek(4, 0); err != nil {
		return fmt.Errorf("failed to seek to chunk size: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, chunkSize); err != nil {
		return fmt.Errorf("failed to write chunk size: %w", err)
	}

	// Seek to data size position and update
	if _, err := w.file.Seek(40, 0); err != nil {
		return fmt.Errorf("failed to seek to data size: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, dataSize); err != nil {
		return fmt.Errorf("failed to write data size: %w", err)
	}

	err := w.file.Close()
	w.file = nil
	return err
}

// writeHeader writes the initial WAV header
func (w *Writer) writeHeader() error {
	// RIFF header
	if _, err := w.file.WriteString("RIFF"); err != nil {
		return err
	}
	
	// Chunk size (will be updated in Close)
	if err := binary.Write(w.file, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	
	// WAVE header
	if _, err := w.file.WriteString("WAVE"); err != nil {
		return err
	}

	// fmt chunk
	if _, err := w.file.WriteString("fmt "); err != nil {
		return err
	}
	
	// fmt chunk size
	if err := binary.Write(w.file, binary.LittleEndian, uint32(16)); err != nil {
		return err
	}
	
	// Audio format (PCM = 1)
	if err := binary.Write(w.file, binary.LittleEndian, uint16(1)); err != nil {
		return err
	}
	
	// Number of channels
	if err := binary.Write(w.file, binary.LittleEndian, w.numChannels); err != nil {
		return err
	}
	
	// Sample rate
	if err := binary.Write(w.file, binary.LittleEndian, w.sampleRate); err != nil {
		return err
	}
	
	// Byte rate
	byteRate := w.sampleRate * uint32(w.numChannels) * uint32(w.bitsPerSample) / 8
	if err := binary.Write(w.file, binary.LittleEndian, byteRate); err != nil {
		return err
	}
	
	// Block align
	blockAlign := w.numChannels * w.bitsPerSample / 8
	if err := binary.Write(w.file, binary.LittleEndian, blockAlign); err != nil {
		return err
	}
	
	// Bits per sample
	if err := binary.Write(w.file, binary.LittleEndian, w.bitsPerSample); err != nil {
		return err
	}

	// data chunk header
	if _, err := w.file.WriteString("data"); err != nil {
		return err
	}
	
	// Data size (will be updated in Close)
	if err := binary.Write(w.file, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}

	return nil
}