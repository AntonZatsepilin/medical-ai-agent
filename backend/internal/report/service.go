package report

import (
	"context"
	"fmt"
	"medical-ai-agent/internal/consultation"
	"strings"
)

type TelegramClient interface {
	SendMessage(chatID int64, text string) error
}

type Service struct {
	tgClient     TelegramClient
	doctorChatID int64
}

func NewService(tg TelegramClient, doctorChatID int64) *Service {
	return &Service{
		tgClient:     tg,
		doctorChatID: doctorChatID,
	}
}

func (s *Service) SendDoctorReport(ctx context.Context, c consultation.Consultation) error {
	var sb strings.Builder
	sb.WriteString("📋 **New Patient Report**\n\n")
	sb.WriteString(fmt.Sprintf("**Patient ID:** %s\n", c.PatientID))
	sb.WriteString(fmt.Sprintf("**Emotional State:** %s\n\n", c.CurrentMood))
	
	sb.WriteString("**Collected Medical Facts:**\n")
	if len(c.ExtractedFacts) == 0 {
		sb.WriteString("- No specific facts extracted.\n")
	}
	for _, fact := range c.ExtractedFacts {
		sb.WriteString(fmt.Sprintf("- *%s*: %s (Confidence: %s)\n", fact.Category, fact.Description, fact.Confidence))
	}

	sb.WriteString("\n**Summary:**\n")
	sb.WriteString("Patient consultation complete. Please review facts above.")

	return s.tgClient.SendMessage(s.doctorChatID, sb.String())
}
