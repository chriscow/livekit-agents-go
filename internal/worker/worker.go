package worker

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// Signal and command type constants
const (
	SignalTypePing     = "ping"
	SignalTypePong     = "pong"
	SignalTypeStartJob = "startJob"
	SignalTypeShutdown = "shutdown"
)

type Worker struct {
	url           string
	token         string
	wsClient      *WebSocketClient
	logger        *slog.Logger
	in            chan *Signal
	out           chan *Command
	mu            sync.RWMutex
	connected     bool
	backoffAttempt int
}

type Config struct {
	URL   string
	Token string
}

func New(config Config, logger *slog.Logger) *Worker {
	return &Worker{
		url:      config.URL,
		token:    config.Token,
		logger:   logger,
		in:       make(chan *Signal, 100),
		out:      make(chan *Command, 100),
		wsClient: NewWebSocketClient(config.URL, config.Token, logger),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("Starting worker", slog.String("url", w.url))

	// Main worker loop with reconnection
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker shutting down")
			return w.shutdown()
		default:
			if err := w.connectAndRun(ctx); err != nil {
				w.logger.Error("Worker connection failed", slog.String("error", err.Error()))
				
				// Exponential backoff with jitter
				if err := w.backoffDelay(ctx); err != nil {
					return err
				}
				continue
			}
		}
	}
}

func (w *Worker) connectAndRun(ctx context.Context) error {
	w.logger.Info("Connecting to LiveKit server")
	
	if err := w.wsClient.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func() {
		if err := w.wsClient.Close(); err != nil {
			w.logger.Error("Error closing WebSocket during cleanup", slog.String("error", err.Error()))
		}
	}()

	w.setConnected(true)
	defer w.setConnected(false)

	// Start goroutines for reading and writing
	readCtx, readCancel := context.WithCancel(ctx)
	defer readCancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	// Start signal reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := w.readSignals(readCtx); err != nil {
			errCh <- fmt.Errorf("read signals: %w", err)
		}
	}()

	// Start command writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := w.writeCommands(readCtx); err != nil {
			errCh <- fmt.Errorf("write commands: %w", err)
		}
	}()

	// Start signal processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.processSignals(readCtx)
	}()

	// Wait for error or context cancellation
	select {
	case err := <-errCh:
		readCancel()
		wg.Wait()
		return err
	case <-ctx.Done():
		readCancel()
		wg.Wait()
		return nil
	}
}

func (w *Worker) readSignals(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			signal, err := w.wsClient.ReadSignal(ctx)
			if err != nil {
				return err
			}

			select {
			case w.in <- signal:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func (w *Worker) writeCommands(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case cmd := <-w.out:
			if err := w.wsClient.WriteCommand(ctx, cmd); err != nil {
				return err
			}
		}
	}
}

func (w *Worker) processSignals(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case signal := <-w.in:
			w.handleSignal(ctx, signal)
		}
	}
}

func (w *Worker) handleSignal(ctx context.Context, signal *Signal) {
	w.logger.Debug("Processing signal", slog.String("type", signal.Type))

	switch signal.Type {
	case SignalTypePing:
		// Respond to ping with pong
		pong := &Command{
			Type: SignalTypePong,
			Data: signal.Data,
		}
		select {
		case w.out <- pong:
		case <-ctx.Done():
		default:
			// Channel is closed or full, skip sending
		}

	case SignalTypeStartJob:
		w.logger.Info("Received start job signal")
		// TODO: Implement job handling in later phases

	case SignalTypeShutdown:
		w.logger.Info("Received shutdown signal")
		// Graceful shutdown will be handled by context cancellation

	default:
		w.logger.Warn("Unknown signal type", slog.String("type", signal.Type))
	}
}

func (w *Worker) backoffDelay(ctx context.Context) error {
	w.mu.Lock()
	w.backoffAttempt++
	attempt := w.backoffAttempt
	w.mu.Unlock()
	
	// Exponential backoff: 1s, 2s, 4s, 8s, up to 10s max
	delay := time.Duration(math.Min(math.Pow(2, float64(attempt-1)), 10)) * time.Second
	
	w.logger.Info("Reconnecting with backoff",
		slog.Int("attempt", attempt),
		slog.Duration("delay", delay))

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) setConnected(connected bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if connected && !w.connected {
		// Reset backoff on successful connection
		w.backoffAttempt = 0
		w.logger.Info("Worker connected successfully")
	}
	
	w.connected = connected
}

func (w *Worker) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.connected
}

func (w *Worker) shutdown() error {
	w.logger.Info("Shutting down worker")
	
	// Close out channel to signal command writers to stop
	// Note: in channel is left open - reading goroutines are managed by context cancellation
	close(w.out)
	
	// Close WebSocket connection
	if err := w.wsClient.Close(); err != nil {
		w.logger.Error("Error closing WebSocket", slog.String("error", err.Error()))
		return err
	}
	
	w.logger.Info("Worker shutdown complete")
	return nil
}