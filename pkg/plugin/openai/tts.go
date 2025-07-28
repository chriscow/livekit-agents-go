package openai

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai/tts"
	"github.com/chriscow/livekit-agents-go/pkg/rtc"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAITTS implements the TTS interface using OpenAI's text-to-speech API
type OpenAITTS struct {
	client *openai.Client
	model  string
	voice  string
}

// newOpenAITTS creates a new OpenAI TTS instance
func newOpenAITTS(config map[string]any) (any, error) {
	var apiKey string
	
	// Get API key from config or environment
	if key, ok := config["api_key"].(string); ok {
		apiKey = key
	} else {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required (set OPENAI_API_KEY environment variable or provide api_key in config)")
	}
	
	model, ok := config["model"].(string)
	if !ok || model == "" {
		model = "tts-1" // default model
	}
	
	voice, ok := config["voice"].(string)
	if !ok || voice == "" {
		voice = "alloy" // default voice
	}
	
	return &OpenAITTS{
		client: openai.NewClient(apiKey),
		model:  model,
		voice:  voice,
	}, nil
}

// Synthesize converts text to audio frames using OpenAI TTS
func (o *OpenAITTS) Synthesize(ctx context.Context, req tts.SynthesizeRequest) (<-chan rtc.AudioFrame, error) {
	log.Printf("🔊 Starting OpenAI TTS synthesis (model: %s, voice: %s)", o.model, o.getVoice(req.Voice))
	start := time.Now()
	
	// Create a channel for audio frames
	frameChan := make(chan rtc.AudioFrame, 10)
	
	go func() {
		defer close(frameChan)
		
		// Use voice from request or default
		voice := o.getVoice(req.Voice)
		
		// Create TTS request
		ttsReq := openai.CreateSpeechRequest{
			Model: openai.SpeechModel(o.model),
			Input: req.Text,
			Voice: openai.SpeechVoice(voice),
		}
		
		// Apply speed if specified
		if req.Speed > 0 {
			ttsReq.Speed = float64(req.Speed)
		}
		
		resp, err := o.client.CreateSpeech(ctx, ttsReq)
		if err != nil {
			log.Printf("❌ OpenAI TTS failed: %v", err)
			return
		}
		defer resp.Close()
		
		// Read audio data in chunks and convert to frames
		const chunkSize = 1024 // 16-bit samples
		buffer := make([]byte, chunkSize*2) // *2 for 16-bit samples
		
		for {
			n, err := resp.Read(buffer)
			if n > 0 {
				// Create audio frame - OpenAI returns MP3, but we need PCM
				// For now, we'll pass the raw data and let the consumer handle decoding
				frame := rtc.AudioFrame{
					Data:              append([]byte(nil), buffer[:n]...),
					SampleRate:        24000, // OpenAI TTS default sample rate
					SamplesPerChannel: n / 2, // 16-bit samples
					NumChannels:       1,     // mono
					Timestamp:         time.Since(start),
				}
				
				select {
				case frameChan <- frame:
				case <-ctx.Done():
					return
				}
			}
			
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Printf("❌ Error reading TTS response: %v", err)
				return
			}
		}
		
		duration := time.Since(start)
		log.Printf("✅ OpenAI TTS synthesis completed (duration: %v)", duration)
	}()
	
	return frameChan, nil
}

// getVoice returns the voice to use, preferring request voice over default
func (o *OpenAITTS) getVoice(requestVoice string) string {
	if requestVoice != "" {
		return requestVoice
	}
	return o.voice
}

// Capabilities returns the OpenAI TTS provider's capabilities
func (o *OpenAITTS) Capabilities() tts.TTSCapabilities {
	return tts.TTSCapabilities{
		Streaming:            false, // Not implementing real streaming yet
		SupportedLanguages:   []string{"en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh"}, // Approximate list
		SupportedVoices:      []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"},
		SampleRates:         []int{24000, 22050},
		SupportsSSML:        false,
		SupportsSpeedControl: true,
		SupportsPitchControl: false,
	}
}