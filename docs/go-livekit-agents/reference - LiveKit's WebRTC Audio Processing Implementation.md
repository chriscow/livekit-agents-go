# LiveKit's WebRTC Audio Processing Implementation

## Acoustic Echo Cancellation (AEC) Implementation

LiveKit implements Acoustic Echo Cancellation through its `AudioProcessingModule` class, which wraps WebRTC's native audio processing capabilities. The system provides comprehensive echo cancellation functionality with the following key components:

**Core AEC Architecture:**
The AEC implementation uses WebRTC's Echo Canceller 3 (AEC3) module through a dual-stream processing approach. [1](#7-0)  The system processes both near-end audio (microphone input) via `process_stream()` and far-end audio (speaker output) via `process_reverse_stream()` methods. [2](#7-1) 

**Configuration Options:**
The AudioProcessingModule supports configurable audio processing features including echo cancellation, noise suppression, high-pass filtering, and automatic gain control. [3](#7-2)  The WebRTC configuration is applied through a structured config system. [4](#7-3) 

**Stream Delay Compensation:**
Critical for effective echo cancellation, the system requires precise timing information about the delay between far-end audio rendering and near-end audio capture. [5](#7-4) 

## Canonical Audio Sample Rates and Channel Layouts

**Primary Sample Rates:**
LiveKit's audio stack uses **48000 Hz (48 kHz)** as the canonical sample rate for most audio processing operations, particularly for Opus codec encoding. [6](#7-5)  Additional supported sample rates include:

- **16000 Hz (16 kHz)**: Commonly used for speech recognition and voice activity detection
- **8000 Hz, 22050 Hz, 24000 Hz, 44100 Hz**: Supported for compatibility with various audio formats and bandwidth requirements [7](#7-6) 

**Audio Frame Requirements:**
All audio processing requires frames to be exactly 10 milliseconds in duration. [8](#7-7)  Audio data is formatted as 16-bit signed integers interleaved by channel. [9](#7-8) 

**Channel Layout Support:**
The system primarily supports mono (1 channel) and stereo (2 channels) configurations. [10](#7-9)  Channel layout information is accessible through the GStreamer integration layer for more complex multi-channel scenarios. [11](#7-10) 

## Platform-Specific Audio Input/Output Handling

**iOS/macOS Implementation:**
On Apple platforms, LiveKit uses a sophisticated dual-approach strategy. On physical devices, it defaults to Apple's Voice-Processing I/O for hardware-accelerated echo cancellation, while WebRTC's software processing serves as a fallback. [12](#7-11)  On simulators, where Apple's Voice-Processing I/O is unavailable, it defaults to WebRTC's software processing. [13](#7-12) 

**Android Implementation:**
The Android implementation utilizes `VOICE_COMMUNICATION` audio source for optimal echo cancellation performance and automatically enables hardware-based acoustic echo cancellation and noise suppression on Android Q (API 29) and later. [14](#7-13)  The system includes comprehensive error handling and state callbacks for both audio recording and playback operations. [15](#7-14) 

**Cross-Platform Consistency:**
LiveKit maintains a consistent `AudioFrame` abstraction across all platforms, providing uniform access to sample rate, channel count, and duration properties. [16](#7-15)  This abstraction enables seamless audio processing regardless of the underlying platform implementation.

## Notes

LiveKit's audio processing architecture demonstrates a sophisticated approach to real-time audio handling, leveraging both WebRTC's proven audio processing capabilities and platform-specific optimizations. The 48 kHz sample rate aligns with modern audio standards and Opus codec requirements, while the 10ms frame duration provides optimal latency-quality balance for real-time communication. The platform-specific implementations ensure optimal performance by utilizing hardware acceleration where available while maintaining software fallbacks for compatibility.
