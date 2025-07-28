package job

import (
	"context"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
)

func TestNewRoom(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name    string
		config  RoomConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: RoomConfig{
				URL:      "wss://test.livekit.io",
				Token:    "test-token",
				RoomName: "test-room",
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			config: RoomConfig{
				Token:    "test-token",
				RoomName: "test-room",
			},
			wantErr: true,
		},
		{
			name: "missing token",
			config: RoomConfig{
				URL:      "wss://test.livekit.io",
				RoomName: "test-room",
			},
			wantErr: true,
		},
		{
			name: "missing room name",
			config: RoomConfig{
				URL:   "wss://test.livekit.io",
				Token: "test-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			room, err := NewRoom(ctx, tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if room == nil {
				t.Error("expected room but got nil")
				return
			}
			
			if room.Events == nil {
				t.Error("events channel should not be nil")
			}
			
			if room.IsConnected() {
				t.Error("new room should not be connected")
			}
			
			// Clean up
			room.Disconnect()
		})
	}
}

func TestEvent_Builders(t *testing.T) {
	event := NewEvent(EventParticipantConnected)
	
	if event.Type != EventParticipantConnected {
		t.Errorf("expected event type %s, got %s", EventParticipantConnected, event.Type)  
	}
	
	if event.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
	
	// Test builder pattern
	participant := &livekit.ParticipantInfo{
		Sid:      "test-sid",
		Identity: "test-identity",
	}
	
	track := &livekit.TrackInfo{
		Sid:  "track-sid",
		Type: livekit.TrackType_AUDIO,
	}
	
	data := []byte("test data")
	metadata := "test metadata"
	
	event = event.
		WithParticipant(participant).
		WithTrack(track).
		WithData(data).
		WithMetadata(metadata)
	
	if event.Participant != participant {
		t.Error("participant should be set")
	}
	
	if event.Track != track {
		t.Error("track should be set")
	}
	
	if string(event.Data) != string(data) {
		t.Errorf("expected data %s, got %s", string(data), string(event.Data))
	}
	
	if event.Metadata != metadata {
		t.Errorf("expected metadata %s, got %s", metadata, event.Metadata)
	}
}

func TestRoom_EventChannelFull(t *testing.T) {
	ctx := context.Background()
	
	// Create room with small buffer
	room, err := NewRoom(ctx, RoomConfig{
		URL:             "wss://test.livekit.io",
		Token:           "test-token", 
		RoomName:        "test-room",
		EventBufferSize: 2, // Very small buffer
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	defer room.Disconnect()
	
	// Fill up the channel
	event1 := NewEvent(EventParticipantConnected)
	event2 := NewEvent(EventParticipantConnected)
	event3 := NewEvent(EventParticipantConnected) // This should be dropped
	
	room.sendEvent(event1)
	room.sendEvent(event2) 
	room.sendEvent(event3) // Should be dropped due to full channel
	
	// Verify first two events are received
	select {
	case receivedEvent := <-room.Events:
		if receivedEvent.Type != EventParticipantConnected {
			t.Errorf("expected event type %s, got %s", EventParticipantConnected, receivedEvent.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive first event")
	}
	
	select {
	case receivedEvent := <-room.Events:
		if receivedEvent.Type != EventParticipantConnected {
			t.Errorf("expected event type %s, got %s", EventParticipantConnected, receivedEvent.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected to receive second event")
	}
	
	// Third event should have been dropped, so channel should be empty now
	select {
	case <-room.Events:
		t.Error("did not expect to receive third event (should have been dropped)")
	case <-time.After(50 * time.Millisecond):
		// Expected - no third event
	}
}

func TestRoom_DisconnectClosesChannel(t *testing.T) {
	ctx := context.Background()
	
	room, err := NewRoom(ctx, RoomConfig{
		URL:      "wss://test.livekit.io",
		Token:    "test-token",
		RoomName: "test-room",
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	
	// Disconnect should close the events channel
	room.Disconnect()
	
	// Channel should be closed (may take a moment due to goroutine cleanup)
	var channelClosed bool
	for i := 0; i < 10; i++ {
		select {
		case event, ok := <-room.Events:
			if !ok {
				channelClosed = true
				break
			}
			t.Errorf("expected channel to be closed, but received event: %v", event)
		case <-time.After(10 * time.Millisecond):
			// Try again
		}
	}
	
	if !channelClosed {
		t.Error("expected channel to be closed within reasonable time")
	}
}

func TestRoom_GetParticipants(t *testing.T) {
	ctx := context.Background()
	
	room, err := NewRoom(ctx, RoomConfig{
		URL:      "wss://test.livekit.io",
		Token:    "test-token",
		RoomName: "test-room",
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	defer room.Disconnect()
	
	// Initially no participants
	participants := room.GetParticipants()
	if len(participants) != 0 {
		t.Errorf("expected 0 participants, got %d", len(participants))
	}
	
	// Simulate adding a participant
	testParticipant := &livekit.ParticipantInfo{
		Sid:      "test-sid",
		Identity: "test-identity",
	}
	
	room.mu.Lock()
	room.participants["test-identity"] = testParticipant
	room.mu.Unlock()
	
	// Should now have one participant
	participants = room.GetParticipants()
	if len(participants) != 1 {
		t.Errorf("expected 1 participant, got %d", len(participants))
	}
	
	if participants["test-identity"] == nil {
		t.Error("expected to find test participant")
	}
	
	if participants["test-identity"].Identity != "test-identity" {
		t.Errorf("expected identity 'test-identity', got %s", participants["test-identity"].Identity)
	}
}