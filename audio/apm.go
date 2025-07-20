package audio

import (
	"fmt"
	"unsafe"

	pb "livekit-agents-go/audio/proto"
	"google.golang.org/protobuf/proto"
)

// AudioProcessingModule provides WebRTC audio processing capabilities including 
// echo cancellation, noise suppression, high-pass filtering, and gain control.
type AudioProcessingModule struct {
	client *FfiClient
	handle uint64
}

// AudioProcessingConfig holds configuration for the AudioProcessingModule
type AudioProcessingConfig struct {
	EchoCancellation  bool
	NoiseSuppression  bool
	HighPassFilter    bool
	AutoGainControl   bool
}

// NewAudioProcessingModule creates a new AudioProcessingModule with the specified configuration
func NewAudioProcessingModule(config AudioProcessingConfig) (*AudioProcessingModule, error) {
	client := NewFfiClient()

	// Create the request
	req := &pb.FfiRequest{
		Message: &pb.FfiRequest_NewApm{
			NewApm: &pb.NewApmRequest{
				EchoCancellerEnabled:    &config.EchoCancellation,
				GainControllerEnabled:   &config.AutoGainControl,
				HighPassFilterEnabled:   &config.HighPassFilter,
				NoiseSuppressionEnabled: &config.NoiseSuppression,
			},
		},
	}

	// Send the request
	respData, err := client.Request(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create APM: %w", err)
	}

	// Parse the response
	var resp pb.FfiResponse
	if err := proto.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal APM response: %w", err)
	}

	// Extract the handle
	newApmResp := resp.GetNewApm()
	if newApmResp == nil {
		return nil, fmt.Errorf("invalid response: missing NewApmResponse")
	}

	apm := newApmResp.GetApm()
	if apm == nil {
		return nil, fmt.Errorf("invalid response: missing OwnedApm")
	}

	handle := apm.GetHandle()
	if handle == nil {
		return nil, fmt.Errorf("invalid response: missing handle")
	}

	return &AudioProcessingModule{
		client: client,
		handle: handle.GetId(),
	}, nil
}

// AudioFrame represents an audio frame for processing
type AudioFrame struct {
	Data              []int16
	SampleRate        uint32
	NumChannels       uint32
	SamplesPerChannel uint32
}

// ProcessStream processes the provided audio frame using the configured audio processing features.
// The input audio frame is modified in-place by the underlying audio processing module 
// (e.g., echo cancellation, noise suppression, etc.).
//
// Important: Audio frames must be exactly 10 ms in duration.
func (apm *AudioProcessingModule) ProcessStream(frame *AudioFrame) error {
	if frame == nil || len(frame.Data) == 0 {
		return fmt.Errorf("frame is nil or has no data")
	}

	// Convert frame data to byte slice
	dataPtr := unsafe.Pointer(&frame.Data[0])
	dataSize := uint32(len(frame.Data) * 2) // int16 = 2 bytes
	dataPtrUint64 := uint64(uintptr(dataPtr))

	req := &pb.FfiRequest{
		Message: &pb.FfiRequest_ApmProcessStream{
			ApmProcessStream: &pb.ApmProcessStreamRequest{
				ApmHandle:   &apm.handle,
				DataPtr:     &dataPtrUint64,
				Size:        &dataSize,
				SampleRate:  &frame.SampleRate,
				NumChannels: &frame.NumChannels,
			},
		},
	}

	respData, err := apm.client.Request(req)
	if err != nil {
		return fmt.Errorf("failed to process stream: %w", err)
	}

	var resp pb.FfiResponse
	if err := proto.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal process stream response: %w", err)
	}

	processResp := resp.GetApmProcessStream()
	if processResp == nil {
		return fmt.Errorf("invalid response: missing ApmProcessStreamResponse")
	}

	if errorMsg := processResp.GetError(); errorMsg != "" {
		return fmt.Errorf("APM process stream error: %s", errorMsg)
	}

	return nil
}

// ProcessReverseStream processes the reverse audio frame (typically used for echo cancellation).
// In an echo cancellation scenario, this method is used to process the "far-end" audio
// prior to mixing or feeding it into the echo canceller.
//
// Important: Audio frames must be exactly 10 ms in duration.
func (apm *AudioProcessingModule) ProcessReverseStream(frame *AudioFrame) error {
	if frame == nil || len(frame.Data) == 0 {
		return fmt.Errorf("frame is nil or has no data")
	}

	// Convert frame data to byte slice
	dataPtr := unsafe.Pointer(&frame.Data[0])
	dataSize := uint32(len(frame.Data) * 2) // int16 = 2 bytes
	dataPtrUint64 := uint64(uintptr(dataPtr))

	req := &pb.FfiRequest{
		Message: &pb.FfiRequest_ApmProcessReverseStream{
			ApmProcessReverseStream: &pb.ApmProcessReverseStreamRequest{
				ApmHandle:   &apm.handle,
				DataPtr:     &dataPtrUint64,
				Size:        &dataSize,
				SampleRate:  &frame.SampleRate,
				NumChannels: &frame.NumChannels,
			},
		},
	}

	respData, err := apm.client.Request(req)
	if err != nil {
		return fmt.Errorf("failed to process reverse stream: %w", err)
	}

	var resp pb.FfiResponse
	if err := proto.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal process reverse stream response: %w", err)
	}

	processResp := resp.GetApmProcessReverseStream()
	if processResp == nil {
		return fmt.Errorf("invalid response: missing ApmProcessReverseStreamResponse")
	}

	if errorMsg := processResp.GetError(); errorMsg != "" {
		return fmt.Errorf("APM process reverse stream error: %s", errorMsg)
	}

	return nil
}

// SetStreamDelayMs sets the delay in ms between ProcessReverseStream() receiving a far-end
// frame and ProcessStream() receiving a near-end frame containing the corresponding echo.
// This must be called if and only if echo processing is enabled.
func (apm *AudioProcessingModule) SetStreamDelayMs(delayMs int32) error {
	req := &pb.FfiRequest{
		Message: &pb.FfiRequest_ApmSetStreamDelay{
			ApmSetStreamDelay: &pb.ApmSetStreamDelayRequest{
				ApmHandle: &apm.handle,
				DelayMs:   &delayMs,
			},
		},
	}

	respData, err := apm.client.Request(req)
	if err != nil {
		return fmt.Errorf("failed to set stream delay: %w", err)
	}

	var resp pb.FfiResponse
	if err := proto.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("failed to unmarshal set stream delay response: %w", err)
	}

	delayResp := resp.GetApmSetStreamDelay()
	if delayResp == nil {
		return fmt.Errorf("invalid response: missing ApmSetStreamDelayResponse")
	}

	if errorMsg := delayResp.GetError(); errorMsg != "" {
		return fmt.Errorf("APM set stream delay error: %s", errorMsg)
	}

	return nil
}

// Close releases the AudioProcessingModule handle
func (apm *AudioProcessingModule) Close() error {
	if apm.handle != 0 {
		err := apm.client.DropHandle(apm.handle)
		apm.handle = 0
		return err
	}
	return nil
}