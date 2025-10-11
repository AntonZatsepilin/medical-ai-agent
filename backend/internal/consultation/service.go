package consultation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AgentClient defines the interface for the AI agent interactions
// We define it here to decouple from the specific agent implementation
type AgentClient interface {
	RunCommunicator(ctx context.Context, history []Message, mood EmotionalState) (string, EmotionalState, error)
	RunAnalyst(ctx context.Context, history []Message) ([]MedicalFact, error)
	RunSupervisor(ctx context.Context, history []Message, facts []MedicalFact) (bool, error)
	GenerateRecommendations(ctx context.Context, facts []MedicalFact) (string, error)
}

// ReportService defines the interface for sending reports
type ReportService interface {
	SendDoctorReport(ctx context.Context, c Consultation) error
}

// TTSClient defines the interface for Text-to-Speech
type TTSClient interface {
	Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error)
}

type Service interface {
	ProcessUserAudio(ctx context.Context, consultationID uuid.UUID, transcribedText string) (string, error)
	CreateConsultation(ctx context.Context, patientID uuid.UUID) (*Consultation, error)
	SynthesizeSpeech(ctx context.Context, text string) ([]byte, error)
}

type service struct {
	repo         Repository
	aiClient     AgentClient
	ttsClient    TTSClient
	reportSvc    ReportService
}

func NewService(repo Repository, ai AgentClient, tts TTSClient, report ReportService) Service {
	return &service{
		repo:      repo,
		aiClient:  ai,
		ttsClient: tts,
		reportSvc: report,
	}
}

func (s *service) SynthesizeSpeech(ctx context.Context, text string) ([]byte, error) {
	// Use a default voice ID or load from config/env if needed
	// For now, we'll let the client use its default or pass empty
	return s.ttsClient.Synthesize(ctx, text, "")
}

func (s *service) CreateConsultation(ctx context.Context, patientID uuid.UUID) (*Consultation, error) {
	c := &Consultation{
		ID:          uuid.New(),
		PatientID:   patientID,
		History:     []Message{},
		CurrentMood: StateNeutral,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Save(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ProcessUserAudio acts as the Central Executive
func (s *service) ProcessUserAudio(ctx context.Context, consultationID uuid.UUID, text string) (string, error) {
	// 1. Load Context (Working Memory)
	consultation, err := s.repo.GetByID(ctx, consultationID)
	if err != nil {
		return "", err
	}

	// 2. Update Episodic Memory (User Input)
	consultation.History = append(consultation.History, Message{
		Role: "user", Content: text, Timestamp: time.Now(),
	})

	// 3. Run Communicator Agent (Synchronous - Fast Path)
	response, newMood, err := s.aiClient.RunCommunicator(ctx, consultation.History, consultation.CurrentMood)
	if err != nil {
		return "", fmt.Errorf("communicator failed: %w", err)
	}

	// Update Episodic Memory (AI Response) & Emotional State
	consultation.History = append(consultation.History, Message{
		Role: "assistant", Content: response, Timestamp: time.Now(),
	})
	consultation.CurrentMood = newMood

	// 4. Save State immediately
	if err := s.repo.Save(ctx, consultation); err != nil {
		return "", err
	}

	// 5. Run Analyst & Supervisor Agents (Asynchronous - Background Processing)
	go func(c Consultation) {
		// Create a detached context for background work
		bgCtx := context.Background()

		// Analyst: Extract Facts
		newFacts, err := s.aiClient.RunAnalyst(bgCtx, c.History)
		if err == nil && len(newFacts) > 0 {
			c.ExtractedFacts = append(c.ExtractedFacts, newFacts...)
		}

		// Supervisor: Check if we are done
		// Only run supervisor if the consultation is not already marked as complete
		if !c.IsComplete {
			isComplete, err := s.aiClient.RunSupervisor(bgCtx, c.History, c.ExtractedFacts)
			if err != nil {
				fmt.Printf("Supervisor error: %v\n", err)
			}
			if err == nil && isComplete {
				fmt.Println("Supervisor decided consultation is complete. Generating recommendations...")
				
				// Generate Recommendations
				recs, err := s.aiClient.GenerateRecommendations(bgCtx, c.ExtractedFacts)
				if err != nil {
					fmt.Printf("Failed to generate recommendations: %v\n", err)
					c.Recommendations = "Не удалось сгенерировать рекомендации."
				} else {
					c.Recommendations = recs
				}

				c.IsComplete = true
				
				// Trigger Report Generation
				if err := s.reportSvc.SendDoctorReport(bgCtx, c); err != nil {
					fmt.Printf("Failed to send report: %v\n", err)
				} else {
					fmt.Println("Report sent successfully.")
				}
			} else {
				fmt.Println("Supervisor decided consultation is NOT complete yet.")
			}
		}

		// Save updated cognitive state
		_ = s.repo.Save(bgCtx, &c)
	}(*consultation)

	return response, nil
}
