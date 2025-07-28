package ai

import (
	"errors"
	"time"
)

// Package ai provides common types and utilities for AI provider implementations.
// It defines standard error types, retry configurations, and helper functions
// used across STT, TTS, LLM, VAD, and audio processing providers.

// Common error types used across AI providers
var (
	// ErrRecoverable indicates a temporary failure that may succeed if retried.
	// Examples: network timeout, rate limiting, temporary service unavailability.
	// Recommended action: retry with exponential backoff.
	ErrRecoverable = errors.New("recoverable AI provider error")

	// ErrFatal indicates a permanent failure that will not succeed if retried.
	// Examples: invalid API key, unsupported format, malformed request.
	// Recommended action: fail fast, do not retry.
	ErrFatal = errors.New("fatal AI provider error")
)

// RetryConfig configures retry behavior for recoverable errors
type RetryConfig struct {
	MaxRetries    int           // Maximum number of retry attempts
	InitialDelay  time.Duration // Initial delay before first retry
	MaxDelay      time.Duration // Maximum delay between retries
	BackoffFactor float64       // Exponential backoff multiplier
	JitterPercent float32       // Random jitter percentage (0.0-1.0)
}

// DefaultRetryConfig provides sensible defaults for AI provider retries
var DefaultRetryConfig = RetryConfig{
	MaxRetries:    3,
	InitialDelay:  100 * time.Millisecond,
	MaxDelay:      5 * time.Second,
	BackoffFactor: 2.0,
	JitterPercent: 0.1,
}

// IsRecoverable checks if an error is recoverable and should be retried
func IsRecoverable(err error) bool {
	return errors.Is(err, ErrRecoverable)
}

// IsFatal checks if an error is fatal and should not be retried
func IsFatal(err error) bool {
	return errors.Is(err, ErrFatal)
}

// RetryableError wraps an underlying error with retry classification
type RetryableError struct {
	Underlying error
	Retryable  bool
	Message    string
}

func (e *RetryableError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Underlying.Error()
}

func (e *RetryableError) Unwrap() error {
	if e.Retryable {
		return ErrRecoverable
	}
	return ErrFatal
}

// NewRecoverableError creates a recoverable error with context
func NewRecoverableError(underlying error, message string) error {
	return &RetryableError{
		Underlying: underlying,
		Retryable:  true,
		Message:    message,
	}
}

// NewFatalError creates a fatal error with context
func NewFatalError(underlying error, message string) error {
	return &RetryableError{
		Underlying: underlying,
		Retryable:  false,
		Message:    message,
	}
}
