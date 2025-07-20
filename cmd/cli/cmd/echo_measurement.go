package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"livekit-agents-go/audio"
	"livekit-agents-go/media"
)

// NewEchoMeasurementCmd creates the echo measurement command
func NewEchoMeasurementCmd() *cobra.Command {
	var (
		inputFile     string
		referenceFile string
		outputDir     string
		sampleRate    int
		enableEcho    bool
		enableNoise   bool
		enableAGC     bool
		delayMs       int
		recordOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "echo-measurement",
		Short: "Measure echo reduction with real audio files",
		Long: `Measure acoustic echo cancellation effectiveness using real audio recordings.

This command processes real audio files to demonstrate echo reduction:
1. Loads microphone input file (with echo)
2. Loads speaker reference file (clean audio that caused the echo)
3. Processes the input through AEC using the reference
4. Measures and reports echo reduction achieved
5. Optionally saves processed audio for listening tests

The input file should contain audio captured from a microphone that includes
echo from speakers playing the reference audio. This simulates real-world
console mode conditions.

Examples:
  # Basic echo measurement
  pipeline-test echo-measurement --input mic_with_echo.wav --reference speaker_output.wav
  
  # Save processed audio for listening test
  pipeline-test echo-measurement --input mic.wav --reference ref.wav --record-output
  
  # Custom AEC settings
  pipeline-test echo-measurement --input mic.wav --reference ref.wav --delay-ms 75 --sample-rate 48000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("--input file is required")
			}
			if referenceFile == "" {
				return fmt.Errorf("--reference file is required")
			}

			return runEchoMeasurement(EchoMeasurementConfig{
				InputFile:     inputFile,
				ReferenceFile: referenceFile,
				OutputDir:     outputDir,
				SampleRate:    sampleRate,
				EnableEcho:    enableEcho,
				EnableNoise:   enableNoise,
				EnableAGC:     enableAGC,
				DelayMs:       delayMs,
				RecordOutput:  recordOutput,
			})
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "input audio file with echo (required)")
	cmd.Flags().StringVarP(&referenceFile, "reference", "r", "", "reference audio file (speaker output) (required)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "./test-results", "directory to save processed audio")
	cmd.Flags().IntVar(&sampleRate, "sample-rate", 24000, "target sample rate for processing")
	cmd.Flags().BoolVar(&enableEcho, "enable-echo", true, "enable echo cancellation")
	cmd.Flags().BoolVar(&enableNoise, "enable-noise", true, "enable noise suppression")
	cmd.Flags().BoolVar(&enableAGC, "enable-agc", true, "enable automatic gain control")
	cmd.Flags().IntVar(&delayMs, "delay-ms", 50, "echo delay in milliseconds")
	cmd.Flags().BoolVar(&recordOutput, "record-output", true, "save processed audio files")

	return cmd
}

// EchoMeasurementConfig holds configuration for echo measurement
type EchoMeasurementConfig struct {
	InputFile     string
	ReferenceFile string
	OutputDir     string
	SampleRate    int
	EnableEcho    bool
	EnableNoise   bool
	EnableAGC     bool
	DelayMs       int
	RecordOutput  bool
}

// runEchoMeasurement performs echo measurement with real audio files
func runEchoMeasurement(config EchoMeasurementConfig) error {
	fmt.Println("🔬 Real-World Echo Measurement")
	fmt.Println("==============================")
	fmt.Printf("🎤 Input file: %s\n", config.InputFile)
	fmt.Printf("🔊 Reference file: %s\n", config.ReferenceFile)
	fmt.Printf("🎵 Target sample rate: %d Hz\n", config.SampleRate)
	fmt.Printf("🎛️  Echo cancellation: %v\n", config.EnableEcho)
	fmt.Printf("🔇 Noise suppression: %v\n", config.EnableNoise)
	fmt.Printf("📈 Auto gain control: %v\n", config.EnableAGC)
	fmt.Printf("⏳ Echo delay: %d ms\n", config.DelayMs)
	fmt.Println()

	// Create output directory
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load audio files
	fmt.Println("📂 Loading audio files...")
	inputAudio, err := loadWAVFile(config.InputFile, config.SampleRate)
	if err != nil {
		return fmt.Errorf("failed to load input file: %w", err)
	}
	fmt.Printf("✅ Loaded input: %d samples, %.2f seconds\n", 
		len(inputAudio.Data)/2, float64(len(inputAudio.Data)/2)/float64(config.SampleRate))

	referenceAudio, err := loadWAVFile(config.ReferenceFile, config.SampleRate)
	if err != nil {
		return fmt.Errorf("failed to load reference file: %w", err)
	}
	fmt.Printf("✅ Loaded reference: %d samples, %.2f seconds\n", 
		len(referenceAudio.Data)/2, float64(len(referenceAudio.Data)/2)/float64(config.SampleRate))

	// Align audio lengths (use shorter length)
	minLength := len(inputAudio.Data)
	if len(referenceAudio.Data) < minLength {
		minLength = len(referenceAudio.Data)
	}
	inputAudio.Data = inputAudio.Data[:minLength]
	referenceAudio.Data = referenceAudio.Data[:minLength]

	fmt.Printf("🔧 Processing %d samples (%.2f seconds)\n", 
		minLength/2, float64(minLength/2)/float64(config.SampleRate))
	fmt.Println()

	// Create AEC processor
	aecConfig := audio.AECConfig{
		EnableEchoCancellation: config.EnableEcho,
		EnableNoiseSuppression: config.EnableNoise,
		EnableAutoGainControl:  config.EnableAGC,
		DelayMs:               config.DelayMs,
		SampleRate:            config.SampleRate,
		Channels:              1,
	}

	processor, err := audio.NewLiveKitAECProcessor(aecConfig)
	if err != nil {
		return fmt.Errorf("failed to create AEC processor: %w", err)
	}
	defer processor.Close()

	frameSize := processor.GetFrameSize()
	fmt.Printf("🎯 Processing with %d sample frames (%.1f ms)\n", 
		frameSize, float64(frameSize)*1000.0/float64(config.SampleRate))
	fmt.Println()

	// Process audio in frames
	fmt.Println("🚀 Processing audio through AEC...")
	
	processedAudio := &AudioData{
		Format: inputAudio.Format,
		Data:   make([]byte, len(inputAudio.Data)),
	}

	frameBytes := frameSize * 2 // 16-bit samples
	totalFrames := len(inputAudio.Data) / frameBytes
	
	var beforeEchoEnergy, afterEchoEnergy float64
	var totalProcessingTime time.Duration
	
	ctx := context.Background()

	for frameIdx := 0; frameIdx < totalFrames; frameIdx++ {
		startByte := frameIdx * frameBytes
		endByte := startByte + frameBytes
		
		if endByte > len(inputAudio.Data) {
			break // Skip incomplete frame
		}

		// Create frames for this chunk
		inputFrame := media.NewAudioFrame(
			inputAudio.Data[startByte:endByte], 
			inputAudio.Format,
		)
		referenceFrame := media.NewAudioFrame(
			referenceAudio.Data[startByte:endByte], 
			referenceAudio.Format,
		)

		// Measure energy before processing
		beforeEnergy := calculateFrameEnergy(inputFrame)
		beforeEchoEnergy += beforeEnergy

		// Process through AEC
		processingStart := time.Now()
		processedFrame, err := processor.ProcessStreams(ctx, inputFrame, referenceFrame)
		processingTime := time.Since(processingStart)
		totalProcessingTime += processingTime

		if err != nil {
			return fmt.Errorf("AEC processing failed on frame %d: %w", frameIdx, err)
		}

		// Measure energy after processing
		afterEnergy := calculateFrameEnergy(processedFrame)
		afterEchoEnergy += afterEnergy

		// Copy processed data
		copy(processedAudio.Data[startByte:endByte], processedFrame.Data)

		// Progress indicator
		if frameIdx%100 == 0 || frameIdx == totalFrames-1 {
			progress := float64(frameIdx+1) / float64(totalFrames) * 100
			fmt.Printf("\r🎯 Progress: %.1f%% (%d/%d frames)", progress, frameIdx+1, totalFrames)
		}
	}

	fmt.Printf("\n✅ Processing completed!\n\n")

	// Calculate results
	avgProcessingTime := totalProcessingTime / time.Duration(totalFrames)
	avgBeforeEnergy := beforeEchoEnergy / float64(totalFrames)
	avgAfterEnergy := afterEchoEnergy / float64(totalFrames)

	// Echo reduction calculation
	var echoReductionDB float64
	if avgAfterEnergy > 0 && avgBeforeEnergy > 0 {
		ratio := avgBeforeEnergy / avgAfterEnergy
		if ratio > 1.0 {
			echoReductionDB = 20.0 * math.Log10(ratio)
		}
	}

	// Spectral analysis for more detailed metrics
	inputSpectralPower := calculateSpectralPower(inputAudio.Data, config.SampleRate)
	processedSpectralPower := calculateSpectralPower(processedAudio.Data, config.SampleRate)
	spectralReduction := 0.0
	if processedSpectralPower > 0 && inputSpectralPower > 0 {
		spectralReduction = 20.0 * math.Log10(inputSpectralPower/processedSpectralPower)
	}

	// Save processed audio if requested
	if config.RecordOutput {
		fmt.Println("💾 Saving processed audio files...")
		
		// Save processed audio
		processedPath := filepath.Join(config.OutputDir, "echo_measurement_processed.wav")
		if err := saveWAVFile(processedPath, processedAudio); err != nil {
			fmt.Printf("⚠️  Warning: Could not save processed audio: %v\n", err)
		} else {
			fmt.Printf("✅ Saved processed audio: %s\n", processedPath)
		}

		// Save original for comparison
		originalPath := filepath.Join(config.OutputDir, "echo_measurement_original.wav")
		if err := saveWAVFile(originalPath, inputAudio); err != nil {
			fmt.Printf("⚠️  Warning: Could not save original audio: %v\n", err)
		} else {
			fmt.Printf("✅ Saved original audio: %s\n", originalPath)
		}

		// Save reference for comparison
		refPath := filepath.Join(config.OutputDir, "echo_measurement_reference.wav")
		if err := saveWAVFile(refPath, referenceAudio); err != nil {
			fmt.Printf("⚠️  Warning: Could not save reference audio: %v\n", err)
		} else {
			fmt.Printf("✅ Saved reference audio: %s\n", refPath)
		}
		fmt.Println()
	}

	// Get AEC statistics
	stats := processor.GetStats()

	// Report results
	fmt.Println("📊 ECHO MEASUREMENT RESULTS")
	fmt.Println("===========================")
	fmt.Printf("🎯 Total frames processed: %d\n", totalFrames)
	fmt.Printf("⚡ Avg processing time: %.2f ms per frame\n", avgProcessingTime.Seconds()*1000)
	fmt.Printf("🎵 Real-time factor: %.3fx\n", 
		avgProcessingTime.Seconds()/(float64(frameSize)/float64(config.SampleRate)))
	fmt.Println()

	fmt.Printf("🔊 Energy before AEC: %.6f\n", avgBeforeEnergy)
	fmt.Printf("🔇 Energy after AEC: %.6f\n", avgAfterEnergy)
	fmt.Printf("📉 Energy-based echo reduction: %.1f dB\n", echoReductionDB)
	fmt.Printf("📊 Spectral power reduction: %.1f dB\n", spectralReduction)
	fmt.Println()

	fmt.Printf("📈 AEC Statistics:\n")
	fmt.Printf("   - Frames processed: %d\n", stats.FramesProcessed)
	fmt.Printf("   - Frames dropped: %d\n", stats.FramesDropped)
	fmt.Printf("   - Configured delay: %d ms\n", stats.DelayMs)
	fmt.Printf("   - Processing latency: %.2f ms\n", stats.ProcessingLatencyMs)
	if stats.EchoReturnLoss > 0 {
		fmt.Printf("   - Echo return loss: %.1f dB\n", stats.EchoReturnLoss)
		fmt.Printf("   - ERL enhancement: %.1f dB\n", stats.EchoReturnLossEnhancement)
	}
	fmt.Println()

	// Assessment
	if avgProcessingTime.Seconds() < (float64(frameSize)/float64(config.SampleRate)) {
		fmt.Println("✅ PERFORMANCE: Real-time capable")
	} else {
		fmt.Println("⚠️  PERFORMANCE: Slower than real-time")
	}

	if echoReductionDB > 20.0 {
		fmt.Println("✅ ECHO REDUCTION: Excellent (>20 dB)")
	} else if echoReductionDB > 10.0 {
		fmt.Println("✅ ECHO REDUCTION: Good (>10 dB)")
	} else if echoReductionDB > 5.0 {
		fmt.Println("⚠️  ECHO REDUCTION: Moderate (>5 dB)")
	} else {
		fmt.Println("❌ ECHO REDUCTION: Poor (<5 dB)")
	}

	if config.RecordOutput {
		fmt.Println()
		fmt.Println("🎧 LISTENING TEST:")
		fmt.Println("Compare the saved audio files to hear the difference:")
		fmt.Printf("   - Original (with echo): %s\n", 
			filepath.Join(config.OutputDir, "echo_measurement_original.wav"))
		fmt.Printf("   - Processed (echo reduced): %s\n", 
			filepath.Join(config.OutputDir, "echo_measurement_processed.wav"))
		fmt.Printf("   - Reference (clean): %s\n", 
			filepath.Join(config.OutputDir, "echo_measurement_reference.wav"))
	}

	return nil
}

// AudioData represents loaded audio file data
type AudioData struct {
	Format media.AudioFormat
	Data   []byte
}

// WAV file header structure
type WAVHeader struct {
	ChunkID       [4]byte  // "RIFF"
	ChunkSize     uint32   // File size - 8
	Format        [4]byte  // "WAVE"
	Subchunk1ID   [4]byte  // "fmt "
	Subchunk1Size uint32   // 16 for PCM
	AudioFormat   uint16   // 1 for PCM
	NumChannels   uint16   // 1 for mono, 2 for stereo
	SampleRate    uint32   // Sample rate
	ByteRate      uint32   // SampleRate * NumChannels * BitsPerSample / 8
	BlockAlign    uint16   // NumChannels * BitsPerSample / 8
	BitsPerSample uint16   // 16 for 16-bit
	Subchunk2ID   [4]byte  // "data"
	Subchunk2Size uint32   // Data size
}

// loadWAVFile loads a WAV file and converts it to the target sample rate
func loadWAVFile(filename string, targetSampleRate int) (*AudioData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read WAV header
	var header WAVHeader
	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read WAV header: %w", err)
	}

	// Validate WAV format
	if string(header.ChunkID[:]) != "RIFF" || string(header.Format[:]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV file")
	}

	if header.AudioFormat != 1 {
		return nil, fmt.Errorf("only PCM format supported, got format %d", header.AudioFormat)
	}

	if header.BitsPerSample != 16 {
		return nil, fmt.Errorf("only 16-bit samples supported, got %d bits", header.BitsPerSample)
	}

	// Read audio data
	audioData := make([]byte, header.Subchunk2Size)
	if _, err := io.ReadFull(file, audioData); err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	// Convert to mono if stereo
	if header.NumChannels == 2 {
		audioData = stereoToMono(audioData)
	} else if header.NumChannels != 1 {
		return nil, fmt.Errorf("unsupported channel count: %d", header.NumChannels)
	}

	// Resample if needed
	if int(header.SampleRate) != targetSampleRate {
		audioData = resampleAudio(audioData, int(header.SampleRate), targetSampleRate)
	}

	format := media.AudioFormat{
		SampleRate:    targetSampleRate,
		Channels:      1,
		BitsPerSample: 16,
		Format:        media.AudioFormatPCM,
	}

	return &AudioData{
		Format: format,
		Data:   audioData,
	}, nil
}

// saveWAVFile saves audio data to a WAV file
func saveWAVFile(filename string, audio *AudioData) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Create WAV header
	header := WAVHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		ChunkSize:     uint32(36 + len(audio.Data)),
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1, // PCM
		NumChannels:   uint16(audio.Format.Channels),
		SampleRate:    uint32(audio.Format.SampleRate),
		ByteRate:      uint32(audio.Format.SampleRate * audio.Format.Channels * audio.Format.BitsPerSample / 8),
		BlockAlign:    uint16(audio.Format.Channels * audio.Format.BitsPerSample / 8),
		BitsPerSample: uint16(audio.Format.BitsPerSample),
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: uint32(len(audio.Data)),
	}

	// Write header
	if err := binary.Write(file, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Write audio data
	if _, err := file.Write(audio.Data); err != nil {
		return fmt.Errorf("failed to write audio data: %w", err)
	}

	return nil
}

// stereoToMono converts stereo audio to mono by averaging channels
func stereoToMono(stereoData []byte) []byte {
	monoData := make([]byte, len(stereoData)/2)
	
	for i := 0; i < len(monoData); i += 2 {
		// Read left and right samples
		left := int16(stereoData[i*2]) | (int16(stereoData[i*2+1]) << 8)
		right := int16(stereoData[i*2+2]) | (int16(stereoData[i*2+3]) << 8)
		
		// Average the channels
		mono := (int32(left) + int32(right)) / 2
		monoSample := int16(mono)
		
		// Write mono sample
		monoData[i] = byte(monoSample & 0xFF)
		monoData[i+1] = byte((monoSample >> 8) & 0xFF)
	}
	
	return monoData
}

// resampleAudio performs simple nearest-neighbor resampling
func resampleAudio(inputData []byte, inputRate, outputRate int) []byte {
	if inputRate == outputRate {
		return inputData
	}

	inputSamples := len(inputData) / 2
	outputSamples := (inputSamples * outputRate) / inputRate
	outputData := make([]byte, outputSamples*2)

	ratio := float64(inputSamples) / float64(outputSamples)

	for i := 0; i < outputSamples; i++ {
		srcIndex := int(float64(i) * ratio)
		if srcIndex >= inputSamples {
			srcIndex = inputSamples - 1
		}

		// Copy sample
		outputData[i*2] = inputData[srcIndex*2]
		outputData[i*2+1] = inputData[srcIndex*2+1]
	}

	return outputData
}

// calculateSpectralPower calculates the spectral power of audio data
func calculateSpectralPower(audioData []byte, sampleRate int) float64 {
	if len(audioData) < 2 {
		return 0.0
	}

	// Convert to float64 samples
	sampleCount := len(audioData) / 2
	samples := make([]float64, sampleCount)
	
	for i := 0; i < sampleCount; i++ {
		sample := int16(audioData[i*2]) | (int16(audioData[i*2+1]) << 8)
		samples[i] = float64(sample) / 32768.0
	}

	// Calculate power spectral density (simplified)
	var power float64
	windowSize := 1024
	if windowSize > sampleCount {
		windowSize = sampleCount
	}

	for i := 0; i < sampleCount-windowSize; i += windowSize {
		windowPower := 0.0
		for j := 0; j < windowSize; j++ {
			windowPower += samples[i+j] * samples[i+j]
		}
		power += windowPower / float64(windowSize)
	}

	return power
}