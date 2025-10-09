package consultation

import (
	"time"

	"github.com/google/uuid"
)

// BICA Components
type EmotionalState string

const (
	StateNeutral  EmotionalState = "neutral"
	StateAnxious  EmotionalState = "anxious"
	StateCalm     EmotionalState = "calm"
	StateCritical EmotionalState = "critical"
)

type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type MedicalFact struct {
	Category    string `json:"category"`    // e.g., "Symptom", "Duration", "Medication"
	Description string `json:"description"` // e.g., "Headache for 3 days"
	Confidence  string `json:"confidence"`  // "High", "Medium", "Low"
}

// Consultation represents the aggregate root
type Consultation struct {
	ID        uuid.UUID `json:"id" db:"id"`
	PatientID uuid.UUID `json:"patient_id" db:"patient_id"`
	
	// Episodic Memory
	History []Message `json:"history" db:"history"`

	// Semantic Memory (The Analyst's Output)
	ExtractedFacts []MedicalFact `json:"facts" db:"facts"`

	// Emotional Module State
	CurrentMood EmotionalState `json:"mood" db:"mood"`

	// Output
	Recommendations string `json:"recommendations" db:"recommendations"`

	// Metacognition Status
	IsComplete bool      `json:"is_complete" db:"is_complete"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
