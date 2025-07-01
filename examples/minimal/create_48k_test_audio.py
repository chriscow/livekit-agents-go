#!/usr/bin/env python3
"""
Generate a 48kHz test audio file for WebRTC compatibility testing.
Creates a 1-second 440Hz sine wave at 48kHz sample rate.
"""

import struct
import math

def create_48k_test_audio():
    """Create 48kHz test audio file"""
    sample_rate = 48000  # 48kHz for WebRTC compatibility
    duration = 1.0  # 1 second
    frequency = 440  # A4 note
    amplitude = 0.3  # Moderate volume
    
    # Calculate number of samples
    num_samples = int(sample_rate * duration)
    
    # Generate sine wave samples
    samples = []
    for i in range(num_samples):
        # Calculate time for this sample
        t = i / sample_rate
        # Generate sine wave value
        value = amplitude * math.sin(2 * math.pi * frequency * t)
        # Convert to 16-bit signed integer
        sample = int(value * 32767)
        samples.append(sample)
    
    # Create WAV file header
    # Reference: http://soundfile.sapp.org/doc/WaveFormat/
    data_size = num_samples * 2  # 2 bytes per sample (16-bit)
    file_size = 36 + data_size
    
    wav_data = bytearray()
    
    # RIFF header
    wav_data.extend(b'RIFF')
    wav_data.extend(struct.pack('<I', file_size))
    wav_data.extend(b'WAVE')
    
    # Format chunk
    wav_data.extend(b'fmt ')
    wav_data.extend(struct.pack('<I', 16))  # Chunk size
    wav_data.extend(struct.pack('<H', 1))   # Audio format (PCM)
    wav_data.extend(struct.pack('<H', 1))   # Number of channels
    wav_data.extend(struct.pack('<I', sample_rate))  # Sample rate
    wav_data.extend(struct.pack('<I', sample_rate * 2))  # Byte rate
    wav_data.extend(struct.pack('<H', 2))   # Block align
    wav_data.extend(struct.pack('<H', 16))  # Bits per sample
    
    # Data chunk
    wav_data.extend(b'data')
    wav_data.extend(struct.pack('<I', data_size))
    
    # Audio samples
    for sample in samples:
        wav_data.extend(struct.pack('<h', sample))
    
    # Write to file
    with open('debug-audio/test-static.wav', 'wb') as f:
        f.write(wav_data)
    
    print(f"✅ Created 48kHz test audio file: debug-audio/test-static.wav")
    print(f"   - Sample rate: {sample_rate} Hz")
    print(f"   - Duration: {duration} seconds")
    print(f"   - Frequency: {frequency} Hz")
    print(f"   - File size: {len(wav_data)} bytes")

if __name__ == "__main__":
    create_48k_test_audio()