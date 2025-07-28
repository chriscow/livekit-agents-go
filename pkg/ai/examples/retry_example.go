package examples

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/chriscow/livekit-agents-go/pkg/ai"
	"github.com/chriscow/livekit-agents-go/pkg/ai/stt"
)

// RetryableSTTClient demonstrates proper error handling and retry logic
// for AI providers with recoverable and fatal error classification.
type RetryableSTTClient struct {
	provider stt.STT
	config   ai.RetryConfig
	logger   *slog.Logger
}

// NewRetryableSTTClient creates a new STT client with retry capabilities
func NewRetryableSTTClient(provider stt.STT, config ai.RetryConfig, logger *slog.Logger) *RetryableSTTClient {
	return &RetryableSTTClient{
		provider: provider,
		config:   config,
		logger:   logger,
	}
}

// NewStreamWithRetry creates an STT stream with automatic retry on recoverable errors
func (c *RetryableSTTClient) NewStreamWithRetry(ctx context.Context, cfg stt.StreamConfig) (stt.STTStream, error) {
	var lastErr error
	
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay with jitter
			delay := c.calculateBackoffDelay(attempt)
			c.logger.Info("Retrying STT stream creation",
				slog.Int("attempt", attempt),
				slog.Duration("delay", delay),
				slog.String("last_error", lastErr.Error()))
			
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		stream, err := c.provider.NewStream(ctx, cfg)
		if err == nil {
			if attempt > 0 {
				c.logger.Info("STT stream creation succeeded after retry",
					slog.Int("attempts", attempt+1))
			}
			return stream, nil
		}

		lastErr = err
		
		// Check error type and decide whether to retry
		if ai.IsFatal(err) {
			c.logger.Error("Fatal error creating STT stream, not retrying",
				slog.String("error", err.Error()),
				slog.Int("attempt", attempt+1))
			return nil, err
		}
		
		if ai.IsRecoverable(err) {
			c.logger.Warn("Recoverable error creating STT stream",
				slog.String("error", err.Error()),
				slog.Int("attempt", attempt+1),
				slog.Int("max_retries", c.config.MaxRetries))
			continue // Retry this error
		}

		// Unknown error type - treat as recoverable for backward compatibility
		c.logger.Warn("Unknown error type, treating as recoverable",
			slog.String("error", err.Error()),
			slog.Int("attempt", attempt+1))
	}

	return nil, fmt.Errorf("exhausted all retry attempts (%d): %w", c.config.MaxRetries, lastErr)
}

// calculateBackoffDelay computes the delay before the next retry attempt
func (c *RetryableSTTClient) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff: delay = initialDelay * (backoffFactor ^ (attempt-1))
	delay := float64(c.config.InitialDelay) * math.Pow(c.config.BackoffFactor, float64(attempt-1))
	
	// Cap at maximum delay
	if delay > float64(c.config.MaxDelay) {
		delay = float64(c.config.MaxDelay)
	}
	
	// Add jitter to avoid thundering herd
	if c.config.JitterPercent > 0 {
		jitterRange := delay * float64(c.config.JitterPercent)
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange // [-jitterRange, +jitterRange]
		delay += jitter
	}
	
	// Ensure non-negative
	if delay < 0 {
		delay = float64(c.config.InitialDelay)
	}
	
	return time.Duration(delay)
}

// ExampleUsage demonstrates how to use the retryable client
func ExampleUsage() {
	// This example shows how to properly handle AI provider errors
	// with automatic retry for recoverable failures.
	
	logger := slog.Default()
	
	// Create your STT provider (fake for example)
	// provider := fake.NewFakeSTT("Example transcript")
	
	// Configure retry behavior
	retryConfig := ai.RetryConfig{
		MaxRetries:      5,                   // Try up to 5 times
		InitialDelay:    200 * time.Millisecond, // Start with 200ms delay
		MaxDelay:        10 * time.Second,    // Cap delays at 10 seconds
		BackoffFactor:   2.0,                 // Double delay each time
		JitterPercent:   0.2,                 // Add ±20% random jitter
	}
	
	// Wrap provider with retry logic -- would use real provider in production
	// client := NewRetryableSTTClient(provider, retryConfig, logger)
	
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Attempt to create stream with automatic retries
	streamConfig := stt.StreamConfig{
		SampleRate:  16000,
		NumChannels: 1,
		Lang:        "en-US",
		// Note: MaxRetry in StreamConfig is for per-request retries,
		// while our RetryableSTTClient handles connection-level retries
		MaxRetry:    1, 
	}
	
	// Log the configuration for demo purposes  
	logger.Info("Example configuration",
		slog.Int("max_retries", retryConfig.MaxRetries),
		slog.Duration("initial_delay", retryConfig.InitialDelay),
		slog.Duration("timeout", 30*time.Second),
		slog.Int("sample_rate", streamConfig.SampleRate))
	
	// Use context to prevent unused variable warning
	_ = ctx
	
	// This would automatically retry on recoverable errors
	// stream, err := client.NewStreamWithRetry(ctx, streamConfig)
	// if err != nil {
	//     if ai.IsFatal(err) {
	//         logger.Error("Fatal STT error - check configuration", slog.String("error", err.Error()))
	//         return
	//     }
	//     logger.Error("STT stream creation failed after all retries", slog.String("error", err.Error()))
	//     return
	// }
	// defer stream.CloseSend()
	
	logger.Info("Example completed - see source code for full implementation details")
}

// Common error handling patterns for different AI providers:
//
// 1. STT Provider Errors:
//    - Recoverable: Network timeout, rate limiting, temporary service unavailability
//    - Fatal: Invalid audio format, unsupported language, authentication failure
//
// 2. TTS Provider Errors:
//    - Recoverable: Service overload, temporary quota exceeded, network issues  
//    - Fatal: Invalid voice ID, unsupported text format, permanent quota exceeded
//
// 3. LLM Provider Errors:
//    - Recoverable: Rate limiting, temporary service error, timeout
//    - Fatal: Invalid API key, unsupported model, content policy violation
//
// 4. VAD Provider Errors:
//    - Recoverable: Processing overload, temporary resource shortage
//    - Fatal: Unsupported audio format, invalid configuration
//
// Best Practices:
// - Always check error type before retrying
// - Use exponential backoff with jitter to avoid thundering herd
// - Log retry attempts for debugging and monitoring
// - Set reasonable timeout contexts to avoid infinite blocking
// - Consider circuit breaker pattern for repeated failures