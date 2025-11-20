package consultation

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AgentClient defines the interface for the AI agent interactions
// We define it here to decouple from the specific agent implementation
type AgentClient interface {
	RunCommunicator(ctx context.Context, history []Message, mood EmotionalState) (string, EmotionalState, error)
	RunCommunicatorStream(ctx context.Context, history []Message, mood EmotionalState) (<-chan string, <-chan error)
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

// STTClient defines the interface for Speech-to-Text
type STTClient interface {
	Transcribe(ctx context.Context, audioData []byte) (string, error)
}

type StreamEvent struct {
	Type string `json:"type"` // "text", "audio", "done", "error"
	Data string `json:"data"`
}

type Service interface {
	ProcessUserAudio(ctx context.Context, consultationID uuid.UUID, transcribedText string) (string, error)
	ProcessUserAudioStream(ctx context.Context, consultationID uuid.UUID, transcribedText string, eventChan chan<- StreamEvent) error
	CreateConsultation(ctx context.Context, patientID uuid.UUID) (*Consultation, error)
	SynthesizeSpeech(ctx context.Context, text string) ([]byte, error)
	TranscribeAudio(ctx context.Context, audioData []byte) (string, error)
}

type service struct {
	repo         Repository
	aiClient     AgentClient
	ttsClient    TTSClient
	sttClient    STTClient
	reportSvc    ReportService
}

func NewService(repo Repository, ai AgentClient, tts TTSClient, stt STTClient, report ReportService) Service {
	return &service{
		repo:      repo,
		aiClient:  ai,
		ttsClient: tts,
		sttClient: stt,
		reportSvc: report,
	}
}

func (s *service) TranscribeAudio(ctx context.Context, audioData []byte) (string, error) {
	return s.sttClient.Transcribe(ctx, audioData)
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

func (s *service) ProcessUserAudioStream(ctx context.Context, consultationID uuid.UUID, text string, eventChan chan<- StreamEvent) error {
	// 1. Load Context
	consultation, err := s.repo.GetByID(ctx, consultationID)
	if err != nil {
		return err
	}

	// 2. Update Episodic Memory (User Input)
	consultation.History = append(consultation.History, Message{
		Role: "user", Content: text, Timestamp: time.Now(),
	})

	// 3. Run Communicator Stream
	tokenChan, errChan := s.aiClient.RunCommunicatorStream(ctx, consultation.History, consultation.CurrentMood)

	var fullResponseBuilder strings.Builder
	var currentSentenceBuilder strings.Builder
	var moodStrBuilder strings.Builder
	inMoodBlock := false
	moodFound := false
	
	// Helper to process sentence audio
	processAudio := func(text string) {
		if len(strings.TrimSpace(text)) == 0 {
			return
		}
		audio, err := s.SynthesizeSpeech(context.Background(), text)
		if err == nil {
			b64 := base64.StdEncoding.EncodeToString(audio)
			eventChan <- StreamEvent{Type: "audio", Data: b64}
		}
	}

	for {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
			// If err is nil (closed), we are done
			goto Done
		case token, ok := <-tokenChan:
			if !ok {
				goto Done
			}

			// Handle Mood Parsing [MOOD: ...]
			if !moodFound {
				if strings.Contains(token, "[") {
					inMoodBlock = true
				}
				if inMoodBlock {
					moodStrBuilder.WriteString(token)
					if strings.Contains(token, "]") {
						inMoodBlock = false
						moodFound = true
						
						// Parse mood
						moodStr := moodStrBuilder.String()
						if strings.HasPrefix(moodStr, "[MOOD:") && strings.HasSuffix(moodStr, "]") {
							m := strings.TrimSuffix(strings.TrimPrefix(moodStr, "[MOOD:"), "]")
							m = strings.TrimSpace(m)
							switch strings.ToLower(m) {
							case "тревожное", "anxious":
								consultation.CurrentMood = StateAnxious
							case "критическое", "critical":
								consultation.CurrentMood = StateCritical
							case "спокойное", "calm", "neutral", "нейтральное":
								consultation.CurrentMood = StateCalm
							}
						}
						continue
					}
					continue
				}
			}

			// Content
			fullResponseBuilder.WriteString(token)
			currentSentenceBuilder.WriteString(token)
			eventChan <- StreamEvent{Type: "text", Data: token}

			// Check for sentence end
			if strings.ContainsAny(token, ".?!") {
				// Simple heuristic: if we have enough chars and punctuation
				sentence := currentSentenceBuilder.String()
				if len(sentence) > 10 {
					processAudio(sentence)
					currentSentenceBuilder.Reset()
				}
			}
		}
	}

Done:
	// Process remaining audio
	remaining := currentSentenceBuilder.String()
	if len(remaining) > 0 {
		processAudio(remaining)
	}

	eventChan <- StreamEvent{Type: "done", Data: ""}

	// Post-processing (Save history, Background agents)
	response := fullResponseBuilder.String()
	consultation.History = append(consultation.History, Message{
		Role: "assistant", Content: response, Timestamp: time.Now(),
	})
	
	if err := s.repo.Save(ctx, consultation); err != nil {
		fmt.Printf("Failed to save consultation: %v\n", err)
	}

	// Check for completion phrases
	forceComplete := false
	lowerResp := strings.ToLower(response)
	if strings.Contains(lowerResp, "врач скоро подойдет") || 
	   strings.Contains(lowerResp, "до свидания") || 
	   strings.Contains(lowerResp, "всего доброго") ||
	   strings.Contains(lowerResp, "ждите врача") {
		forceComplete = true
	}

	// Background agents
	go func(c Consultation) {
		bgCtx := context.Background()
		newFacts, err := s.aiClient.RunAnalyst(bgCtx, c.History)
		if err == nil && len(newFacts) > 0 {
			c.ExtractedFacts = append(c.ExtractedFacts, newFacts...)
		}

		if !c.IsComplete {
			isComplete := false
			var err error

			if forceComplete {
				isComplete = true
			} else {
				isComplete, err = s.aiClient.RunSupervisor(bgCtx, c.History, c.ExtractedFacts)
			}

			if err == nil && isComplete {
				recs, err := s.aiClient.GenerateRecommendations(bgCtx, c.ExtractedFacts)
				if err == nil {
					c.Recommendations = recs
				}
				c.IsComplete = true
				s.reportSvc.SendDoctorReport(bgCtx, c)
			}
		}
		_ = s.repo.Save(bgCtx, &c)
	}(*consultation)

	return nil
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

	// Check for completion phrases to force finish the consultation
	// This ensures that if the AI says "Doctor is coming", we definitely send the report.
	forceComplete := false
	lowerResp := strings.ToLower(response)
	if strings.Contains(lowerResp, "врач скоро подойдет") || 
	   strings.Contains(lowerResp, "до свидания") || 
	   strings.Contains(lowerResp, "всего доброго") ||
	   strings.Contains(lowerResp, "ждите врача") {
		forceComplete = true
		fmt.Println("Detected completion phrase in assistant response. Forcing completion.")
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
			isComplete := false
			var err error

			if forceComplete {
				isComplete = true
				fmt.Println("Forcing completion based on assistant response.")
			} else {
				isComplete, err = s.aiClient.RunSupervisor(bgCtx, c.History, c.ExtractedFacts)
			}

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
