# Audio Test Suite

A comprehensive test program to validate the PortAudio implementation and audio pipeline components.

## Overview

This test suite validates all aspects of the LiveKit Agents Go audio system:

- **Device Discovery**: Enumerate and validate audio devices
- **Microphone Capture**: Test audio input from microphone
- **Speaker Playback**: Test audio output to speakers  
- **Loopback Testing**: Microphone to speaker real-time testing
- **Pipeline Integration**: Full VAD → STT → LLM → TTS chain testing

## Prerequisites

- PortAudio system library installed (`brew install portaudio` on macOS)
- Go 1.21+ with all project dependencies
- Working microphone and speakers/headphones
- Quiet environment for audio testing

## Usage

```bash
cd examples/audio-test
go run main.go
```

## Test Menu

```
🎧 AUDIO TEST SUITE
============================================================
1. 🔍 Device Discovery Test
2. 🎤 Microphone Test  
3. 🔊 Speaker Test
4. 🔄 Loopback Test (Mic → Speaker)
5. 🤖 Pipeline Integration Test
6. 🚀 Run All Tests
7. 📊 Show Test Summary
0. 👋 Exit
```

## Individual Tests

### 1. Device Discovery Test
- Lists all available audio input/output devices
- Shows device capabilities (channels, sample rates)
- Identifies default input/output devices
- **Expected Result**: Displays your system's audio devices

### 2. Microphone Test  
- Captures 5 seconds of audio from microphone
- Calculates and displays audio energy levels
- Tests audio format handling and frame processing
- **User Action**: Speak or make noise during recording
- **Expected Result**: Non-zero audio energy levels detected

### 3. Speaker Test
- Plays test tones at 220Hz, 440Hz, and 880Hz
- Tests audio format conversion and playback
- Validates speaker output functionality
- **User Action**: Listen for clear tones
- **Expected Result**: Three distinct musical tones (A3, A4, A5)

### 4. Loopback Test
- Real-time microphone to speaker routing
- 10-second live audio passthrough
- Tests full audio I/O pipeline under load
- **User Action**: Speak and listen for your voice
- **Expected Result**: Hear your voice with slight delay

### 5. Pipeline Integration Test
- Tests VAD, STT, LLM, TTS services with mock implementations
- Validates service plugin system
- Tests complete voice processing workflow
- Measures pipeline latency
- **Expected Result**: All pipeline components process successfully

### 6. Run All Tests
- Executes tests 1-5 in sequence
- Provides comprehensive system validation
- **Recommended** for initial setup validation

## Test Results

The test suite provides:

- ✅ **PASS** / ❌ **FAIL** status for each test
- Execution time for performance monitoring  
- Detailed error messages for debugging
- Audio energy levels and technical metrics
- Final summary with pass/fail counts

## Troubleshooting

### No Audio Devices Found
- Ensure PortAudio is installed: `brew install portaudio`
- Check system audio settings
- Restart the application

### Microphone Test Fails
- Check microphone permissions in System Preferences
- Verify microphone is not muted
- Try speaking louder during the test
- Check for conflicting applications using microphone

### Speaker Test Fails  
- Verify speakers/headphones are connected
- Check system volume is not muted
- Test with different audio output device
- Ensure no other applications are using audio output

### Low Audio Energy
- Speak closer to microphone
- Check microphone sensitivity settings
- Ensure quiet background environment
- Verify microphone is working in other applications

### Pipeline Test Fails
- Check that mock services are properly registered
- Verify all service dependencies are available
- Review error messages for specific component failures

## Technical Details

- **Audio Format**: 48kHz, 16-bit, mono PCM by default
- **Frame Size**: 1024 samples per frame (≈21ms at 48kHz)
- **Test Tones**: Pure sine waves at musical frequencies
- **Mock Services**: Realistic test implementations for offline validation
- **Cross-Platform**: Works on macOS, Linux, Windows with PortAudio

## Expected Output Example

```
🔍 Running Device Discovery Test...
Available audio devices (4):
  Device 0: Studio Display Microphone
    Max input channels: 1
    Max output channels: 0
    Default sample rate: 48000 Hz
  Device 1: Studio Display Speakers
    Max input channels: 0
    Max output channels: 8
    Default sample rate: 48000 Hz
  ...
✅ PASS - Device Discovery (45ms)

🎤 Running Microphone Test...
🔴 Recording... make some noise!
📊 Energy level:  0.023
📊 Energy level:  0.156
...
✅ PASS - Microphone Capture (5.1s)
   📝 Captured 47 audio frames
   📊 Average audio energy: 0.089
```

This test suite ensures your audio foundation is solid before building more complex voice processing features.