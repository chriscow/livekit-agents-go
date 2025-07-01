package agents

import (
	"context"
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"
)

// JobContext provides execution context for agents (equivalent to Python JobContext)
type JobContext struct {
	// Room connection and management
	Room *lksdk.Room
	
	// Job process information
	Process *JobProcess
	
	// Agent session for voice pipeline
	Session *AgentSession
	
	// User data and metadata
	UserData map[string]interface{}
	
	// Cancellation context
	Context context.Context
	
	// Entrypoint function to execute
	EntrypointFunc func(ctx *JobContext) error
}

// JobProcess manages individual agent execution
type JobProcess struct {
	ID           string
	ExecutorType JobExecutorType
	UserData     map[string]interface{}
	StartTime    time.Time
	Status       JobStatus
}

type JobStatus int

const (
	JobStatusPending JobStatus = iota
	JobStatusRunning
	JobStatusCompleted
	JobStatusFailed
	JobStatusCancelled
)

type JobExecutorType int

const (
	JobExecutorThread JobExecutorType = iota
	JobExecutorProcess
)

// NewJobContext creates a new job context
func NewJobContext(ctx context.Context) *JobContext {
	return &JobContext{
		Context:  ctx,
		UserData: make(map[string]interface{}),
		Process: &JobProcess{
			UserData:  make(map[string]interface{}),
			StartTime: time.Now(),
			Status:    JobStatusPending,
		},
	}
}

// ConnectToRoom connects to a LiveKit room with proper job context
func (jc *JobContext) ConnectToRoom(url, token string) error {
	// TODO: Use proper LiveKit SDK connection once API is stabilized
	// For now, this is a placeholder that demonstrates the intended interface
	room, err := lksdk.ConnectToRoom(url, lksdk.ConnectInfo{
		APIKey:    "", // Would be set from worker options
		APISecret: "", // Would be set from worker options
	}, &lksdk.RoomCallback{
		// Room event callbacks would be set here
	})
	
	if err != nil {
		return err
	}
	
	jc.Room = room
	return nil
}

// SetUserData sets user data on the job context
func (jc *JobContext) SetUserData(key string, value interface{}) {
	jc.UserData[key] = value
}

// GetUserData gets user data from the job context
func (jc *JobContext) GetUserData(key string) interface{} {
	return jc.UserData[key]
}

// UpdateStatus updates the job process status
func (jp *JobProcess) UpdateStatus(status JobStatus) {
	jp.Status = status
}

// IsRunning returns true if the job is currently running
func (jp *JobProcess) IsRunning() bool {
	return jp.Status == JobStatusRunning
}

// IsCompleted returns true if the job has completed
func (jp *JobProcess) IsCompleted() bool {
	return jp.Status == JobStatusCompleted || jp.Status == JobStatusFailed || jp.Status == JobStatusCancelled
}

// Duration returns the duration the job has been running
func (jp *JobProcess) Duration() time.Duration {
	return time.Since(jp.StartTime)
}