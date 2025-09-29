package agent

import (
	"context"
	"medical-ai-agent/internal/consultation"
)

type DeepSeekClient interface {
	RunCommunicator(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (string, consultation.EmotionalState, error)
	RunAnalyst(ctx context.Context, history []consultation.Message) ([]consultation.MedicalFact, error)
	RunSupervisor(ctx context.Context, history []consultation.Message, facts []consultation.MedicalFact) (bool, error)
}

type client struct {
	apiKey string
}

func NewDeepSeekClient(apiKey string) DeepSeekClient {
	return &client{apiKey: apiKey}
}

// RunCommunicator simulates the persona agent
func (c *client) RunCommunicator(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (string, consultation.EmotionalState, error) {
	// In a real implementation, this would call the DeepSeek Chat Completion API
	// with a system prompt tailored for empathy.
	
	// Example System Prompt:
	// "Ты — эмпатичный медицинский ассистент. Текущее настроение пациента: [mood]. Цель: Успокоить..."
	
	// Mock response for now
	return "Я понимаю, как вы себя чувствуете. Расскажите подробнее, когда началась боль?", consultation.StateCalm, nil
}

// RunAnalyst simulates the cognitive observer
func (c *client) RunAnalyst(ctx context.Context, history []consultation.Message) ([]consultation.MedicalFact, error) {
	// This would analyze the latest messages and extract structured data.
	
	// Mock extraction
	return []consultation.MedicalFact{
		{Category: "Симптом", Description: "Головная боль", Confidence: "Высокая"},
	}, nil
}

// RunSupervisor simulates the metacognition
func (c *client) RunSupervisor(ctx context.Context, history []consultation.Message, facts []consultation.MedicalFact) (bool, error) {
	// Decides if we have enough info.
	if len(facts) > 3 {
		return true, nil
	}
	return false, nil
}
