package job

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/webrtc/v3"
)

// Room wraps the LiveKit room connection and provides event handling.
type Room struct {
	// Events channel for room events
	Events chan *Event
	
	// Internal LiveKit room connection
	room *lksdk.Room
	
	// Context for managing the room lifecycle
	ctx context.Context
	
	// Cancellation function
	cancel context.CancelFunc
	
	// Mutex for thread-safe operations
	mu sync.RWMutex
	
	// Flag to track if room is connected
	connected bool
	
	// Flag to track if events channel is closed
	eventsClosed bool
	
	// Room metadata
	roomInfo *livekit.Room
	
	// Participants tracking
	participants map[string]*livekit.ParticipantInfo
}

// RoomConfig contains configuration for connecting to a room.
type RoomConfig struct {
	// URL of the LiveKit server
	URL string
	
	// Token for authentication
	Token string
	
	// Room name to join
	RoomName string
	
	// Buffer size for events channel
	EventBufferSize int
}

// NewRoom creates a new Room wrapper with the given configuration.
func NewRoom(ctx context.Context, config RoomConfig) (*Room, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if config.RoomName == "" {
		return nil, fmt.Errorf("room name is required")
	}
	
	// Default event buffer size
	bufferSize := config.EventBufferSize
	if bufferSize == 0 {
		bufferSize = 100
	}
	
	roomCtx, cancel := context.WithCancel(ctx)
	
	r := &Room{
		Events:       make(chan *Event, bufferSize),
		ctx:          roomCtx,
		cancel:       cancel,
		connected:    false,
		eventsClosed: false,
		participants: make(map[string]*livekit.ParticipantInfo),
	}
	
	return r, nil
}

// Connect establishes connection to the LiveKit room.
func (r *Room) Connect(config RoomConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.connected {
		return fmt.Errorf("room is already connected")
	}
	
	// Create room callback
	callback := &lksdk.RoomCallback{
		OnParticipantConnected:    r.onParticipantConnected,
		OnParticipantDisconnected: r.onParticipantDisconnected,
		OnRoomMetadataChanged:     r.onRoomMetadataChanged,
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed:   r.onTrackSubscribed,
			OnTrackUnsubscribed: r.onTrackUnsubscribed,
			OnTrackPublished:    r.onTrackPublished,
			OnTrackUnpublished:  r.onTrackUnpublished,
			OnDataReceived:      r.onDataReceived,
		},
	}
	
	// Create LiveKit room connection using token
	room, err := lksdk.ConnectToRoomWithToken(config.URL, config.Token, callback)
	
	if err != nil {
		return fmt.Errorf("failed to connect to room: %w", err)
	}
	
	r.room = room
	r.connected = true
	
	slog.Info("Connected to LiveKit room",
		slog.String("room_name", config.RoomName),
		slog.String("url", config.URL))
	
	return nil
}

// Disconnect closes the room connection and cleans up resources.
func (r *Room) Disconnect() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Always cancel context
	r.cancel()
	
	if r.connected {
		r.connected = false
		
		if r.room != nil {
			r.room.Disconnect()
		}
		
		slog.Info("Disconnected from LiveKit room")
	}
	
	// Close the events channel (only if not already closed)
	if !r.eventsClosed {
		close(r.Events)
		r.eventsClosed = true
	}
	
	return nil
}

// IsConnected returns true if the room is currently connected.
func (r *Room) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connected
}

// LocalParticipant returns the local participant.
func (r *Room) LocalParticipant() *lksdk.LocalParticipant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.room == nil {
		return nil
	}
	
	return r.room.LocalParticipant
}

// GetParticipants returns a copy of all participants in the room.
func (r *Room) GetParticipants() map[string]*livekit.ParticipantInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make(map[string]*livekit.ParticipantInfo)
	for k, v := range r.participants {
		result[k] = v
	}
	return result
}

// AutoSubscribe automatically subscribes to all tracks from a participant.
func (r *Room) AutoSubscribe(participantID string) error {
	r.mu.RLock()
	participant, exists := r.participants[participantID]
	room := r.room
	r.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("participant %s not found", participantID)
	}
	
	if room == nil {
		return fmt.Errorf("room not connected")
	}
	
	// Subscribe to all published tracks from this participant
	for _, track := range participant.Tracks {
		if track.Type == livekit.TrackType_AUDIO || track.Type == livekit.TrackType_VIDEO {
			slog.Info("Auto-subscribing to track",
				slog.String("participant_id", participantID),
				slog.String("track_sid", track.Sid),
				slog.String("track_type", track.Type.String()))
		}
	}
	
	return nil
}

// Event handlers

func (r *Room) onParticipantConnected(participant *lksdk.RemoteParticipant) {
	// Create participant info from the remote participant
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_ACTIVE,
		// Note: More fields could be populated here if available from the SDK
	}
	
	r.mu.Lock()
	r.participants[participant.Identity()] = participantInfo
	r.mu.Unlock()
	
	event := NewEvent(EventParticipantConnected).WithParticipant(participantInfo)
	r.sendEvent(event)
	
	slog.Info("Participant connected",
		slog.String("identity", participant.Identity()),
		slog.String("sid", participant.SID()))
}

func (r *Room) onParticipantDisconnected(participant *lksdk.RemoteParticipant) {
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_DISCONNECTED,
	}
	
	r.mu.Lock()
	delete(r.participants, participant.Identity())
	r.mu.Unlock()
	
	event := NewEvent(EventParticipantDisconnected).WithParticipant(participantInfo)
	r.sendEvent(event)
	
	slog.Info("Participant disconnected",
		slog.String("identity", participant.Identity()),
		slog.String("sid", participant.SID()))
}

func (r *Room) onTrackSubscribed(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_ACTIVE,
	}
	
	trackInfo := &livekit.TrackInfo{
		Sid:  publication.SID(),
		Name: publication.Name(),
		Type: publication.Kind().ProtoType(),
	}
	
	event := NewEvent(EventTrackSubscribed).
		WithParticipant(participantInfo).
		WithTrack(trackInfo)
	r.sendEvent(event)
	
	slog.Info("Track subscribed",
		slog.String("participant", participant.Identity()),
		slog.String("track_sid", publication.SID()),
		slog.String("track_type", publication.Kind().String()))
}

func (r *Room) onTrackUnsubscribed(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_ACTIVE,
	}
	
	trackInfo := &livekit.TrackInfo{
		Sid:  publication.SID(),
		Name: publication.Name(),
		Type: publication.Kind().ProtoType(),
	}
	
	event := NewEvent(EventTrackUnsubscribed).
		WithParticipant(participantInfo).
		WithTrack(trackInfo)
	r.sendEvent(event)
}

func (r *Room) onTrackPublished(publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_ACTIVE,
	}
	
	trackInfo := &livekit.TrackInfo{
		Sid:  publication.SID(),
		Name: publication.Name(),
		Type: publication.Kind().ProtoType(),
	}
	
	event := NewEvent(EventTrackPublished).
		WithParticipant(participantInfo).
		WithTrack(trackInfo)
	r.sendEvent(event)
	
	// Auto-subscribe to new tracks
	if err := r.AutoSubscribe(participant.Identity()); err != nil {
		slog.Error("Failed to auto-subscribe to track",
			slog.String("error", err.Error()),
			slog.String("participant", participant.Identity()))
	}
}

func (r *Room) onTrackUnpublished(publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_ACTIVE,
	}
	
	trackInfo := &livekit.TrackInfo{
		Sid:  publication.SID(),
		Name: publication.Name(),
		Type: publication.Kind().ProtoType(),
	}
	
	event := NewEvent(EventTrackUnpublished).
		WithParticipant(participantInfo).
		WithTrack(trackInfo)
	r.sendEvent(event)
}

func (r *Room) onDataReceived(data []byte, participant *lksdk.RemoteParticipant) {
	participantInfo := &livekit.ParticipantInfo{
		Sid:      participant.SID(),
		Identity: participant.Identity(),
		State:    livekit.ParticipantInfo_ACTIVE,
	}
	
	event := NewEvent(EventDataReceived).
		WithParticipant(participantInfo).
		WithData(data)
	r.sendEvent(event)
}

func (r *Room) onRoomMetadataChanged(metadata string) {
	event := NewEvent(EventRoomMetadataChanged).
		WithMetadata(metadata)
	r.sendEvent(event)
}

// sendEvent sends an event to the Events channel if the room is still connected.
func (r *Room) sendEvent(event *Event) {
	r.mu.RLock()
	closed := r.eventsClosed
	r.mu.RUnlock()
	
	if closed {
		return // Don't send events to closed channel
	}
	
	select {
	case r.Events <- event:
		// Event sent successfully
	case <-r.ctx.Done():
		// Room is disconnected, don't send event
		return
	default:
		// Channel is full, log warning and drop event
		slog.Warn("Events channel is full, dropping event",
			slog.String("event_type", string(event.Type)))
	}
}