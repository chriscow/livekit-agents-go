package job

import (
	"time"

	"github.com/livekit/protocol/livekit"
)

// EventType represents the type of room event.
type EventType string

const (
	// EventParticipantConnected is fired when a participant joins the room
	EventParticipantConnected EventType = "participant_connected"
	
	// EventParticipantDisconnected is fired when a participant leaves the room
	EventParticipantDisconnected EventType = "participant_disconnected"
	
	// EventTrackSubscribed is fired when a track is subscribed
	EventTrackSubscribed EventType = "track_subscribed"
	
	// EventTrackUnsubscribed is fired when a track is unsubscribed
	EventTrackUnsubscribed EventType = "track_unsubscribed"
	
	// EventTrackPublished is fired when a participant publishes a track
	EventTrackPublished EventType = "track_published"
	
	// EventTrackUnpublished is fired when a participant unpublishes a track
	EventTrackUnpublished EventType = "track_unpublished"
	
	// EventDataReceived is fired when data is received from a participant
	EventDataReceived EventType = "data_received"
	
	// EventRoomMetadataChanged is fired when room metadata changes
	EventRoomMetadataChanged EventType = "room_metadata_changed"
)

// Event represents a room event with associated data.
type Event struct {
	// Type of the event
	Type EventType
	
	// Timestamp when the event occurred
	Timestamp time.Time
	
	// Participant associated with the event (if applicable)
	Participant *livekit.ParticipantInfo
	
	// Track associated with the event (if applicable)
	Track *livekit.TrackInfo
	
	// Data payload for data events
	Data []byte
	
	// Metadata for metadata change events
	Metadata string
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType) *Event {
	return &Event{
		Type:      eventType,
		Timestamp: time.Now(),
	}
}

// WithParticipant adds participant information to the event.
func (e *Event) WithParticipant(participant *livekit.ParticipantInfo) *Event {
	e.Participant = participant
	return e
}

// WithTrack adds track information to the event.
func (e *Event) WithTrack(track *livekit.TrackInfo) *Event {
	e.Track = track
	return e
}

// WithData adds data payload to the event.
func (e *Event) WithData(data []byte) *Event {
	e.Data = data
	return e
}

// WithMetadata adds metadata to the event.
func (e *Event) WithMetadata(metadata string) *Event {
	e.Metadata = metadata
	return e
}