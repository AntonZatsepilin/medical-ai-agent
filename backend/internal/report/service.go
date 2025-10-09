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
	sb.WriteString("📋 **Новый отчет о пациенте**\n\n")
	sb.WriteString(fmt.Sprintf("**ID Пациента:** %s\n", c.PatientID))
	sb.WriteString(fmt.Sprintf("**Эмоциональное состояние:** %s\n\n", translateMood(c.CurrentMood)))
	
	sb.WriteString("**Собранные медицинские факты:**\n")
	if len(c.ExtractedFacts) == 0 {
		sb.WriteString("- Факты не выявлены.\n")
	}
	for _, fact := range c.ExtractedFacts {
		sb.WriteString(fmt.Sprintf("- *%s*: %s (Уверенность: %s)\n", fact.Category, fact.Description, fact.Confidence))
	}

	sb.WriteString("\n**Итог:**\n")
	sb.WriteString("Опрос пациента завершен. Пожалуйста, ознакомьтесь с фактами выше.\n\n")

	if c.Recommendations != "" {
		sb.WriteString("**Рекомендации AI:**\n")
		sb.WriteString(c.Recommendations)
	}

	return s.tgClient.SendMessage(s.doctorChatID, sb.String())
}

func translateMood(mood consultation.EmotionalState) string {
	switch mood {
	case consultation.StateAnxious:
		return "Тревожное"
	case consultation.StateCritical:
		return "Критическое"
	case consultation.StateCalm:
		return "Спокойное"
	case consultation.StateNeutral:
		return "Нейтральное"
	default:
		return string(mood)
	}
}
