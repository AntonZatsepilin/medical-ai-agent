package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"medical-ai-agent/internal/consultation"
	"net/http"
	"strings"
	"time"
)

const deepSeekAPIURL = "https://api.deepseek.com/chat/completions"

type DeepSeekClient interface {
	RunCommunicator(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (string, consultation.EmotionalState, error)
	RunAnalyst(ctx context.Context, history []consultation.Message) ([]consultation.MedicalFact, error)
	RunSupervisor(ctx context.Context, history []consultation.Message, facts []consultation.MedicalFact) (bool, error)
}

type client struct {
	apiKey     string
	httpClient *http.Client
}

func NewDeepSeekClient(apiKey string) DeepSeekClient {
	return &client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- API Structures ---

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Format      *jsonFormat   `json:"response_format,omitempty"`
}

type jsonFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// --- Implementations ---

func (c *client) RunCommunicator(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (string, consultation.EmotionalState, error) {
	systemPrompt := fmt.Sprintf(`Ты — эмпатичный медицинский ассистент в приемном отделении.
Твоя задача: вежливо и мягко опросить пациента о его симптомах, пока он ожидает врача.
Текущее настроение пациента (по твоей оценке): %s.

ИНСТРУКЦИЯ ПО ФОРМАТУ ОТВЕТА:
1. Сначала оцени настроение пациента (Спокойное/Тревожное/Критическое).
2. Напиши ответ пациенту.
3. Формат вывода: "[MOOD: <настроение>] <Текст ответа>"

Пример: "[MOOD: Тревожное] Не волнуйтесь, врач скоро подойдет. Скажите, боль острая или тупая?"

Подстраивайся под тон пациента. Задавай по одному вопросу за раз. Не ставь диагнозы.`, mood)

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	for _, msg := range history {
		messages = append(messages, chatMessage{Role: msg.Role, Content: msg.Content})
	}

	resp, err := c.makeRequest(ctx, messages, 0.7, false)
	if err != nil {
		return "", consultation.StateNeutral, err
	}

	// Parse Mood and Content
	newMood := mood
	content := resp

	if strings.HasPrefix(resp, "[MOOD:") {
		endIdx := strings.Index(resp, "]")
		if endIdx != -1 {
			moodStr := resp[7:endIdx]
			content = strings.TrimSpace(resp[endIdx+1:])
			
			// Map string to EmotionalState
			switch strings.ToLower(strings.TrimSpace(moodStr)) {
			case "тревожное", "anxious":
				newMood = consultation.StateAnxious
			case "критическое", "critical":
				newMood = consultation.StateCritical
			default:
				newMood = consultation.StateCalm
			}
		}
	}

	return content, newMood, nil
}

func (c *client) RunAnalyst(ctx context.Context, history []consultation.Message) ([]consultation.MedicalFact, error) {
	systemPrompt := `Ты — медицинский аналитик. Твоя задача — извлекать факты из диалога.
Верни ТОЛЬКО валидный JSON массив объектов. Не пиши ничего кроме JSON.
Формат: [{"category": "Симптом/Лекарство/Хронология", "description": "...", "confidence": "Высокая/Средняя/Низкая"}]
Если новых фактов нет, верни пустой массив [].`

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	// Only analyze last few messages to save tokens and focus on recent context
	startIdx := 0
	if len(history) > 6 {
		startIdx = len(history) - 6
	}
	for _, msg := range history[startIdx:] {
		messages = append(messages, chatMessage{Role: msg.Role, Content: msg.Content})
	}

	resp, err := c.makeRequest(ctx, messages, 0.1, true)
	if err != nil {
		return nil, err
	}

	// Clean up response if it contains markdown code blocks
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var facts []consultation.MedicalFact
	if err := json.Unmarshal([]byte(resp), &facts); err != nil {
		// If JSON fails, just return empty to not break flow
		fmt.Printf("Analyst JSON error: %v. Response: %s\n", err, resp)
		return []consultation.MedicalFact{}, nil
	}

	return facts, nil
}

func (c *client) RunSupervisor(ctx context.Context, history []consultation.Message, facts []consultation.MedicalFact) (bool, error) {
	if len(history) < 4 {
		return false, nil
	}

	factsSummary := ""
	for _, f := range facts {
		factsSummary += fmt.Sprintf("- %s: %s\n", f.Category, f.Description)
	}

	systemPrompt := fmt.Sprintf(`Ты — супервайзер медицинского опроса.
Собранные факты:
%s
Твоя задача — решить, достаточно ли информации для формирования первичного отчета врачу.
Критерии завершения:
1. Понятна основная жалоба (что болит).
2. Понятна длительность симптомов (как долго).
3. Есть хотя бы 3 факта в списке.

Если критерии выполнены, отвечай "ДА". Если нужно задать еще вопросы, отвечай "НЕТ".
Ответь ТОЛЬКО словом "ДА" или "НЕТ".`, factsSummary)

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	
	resp, err := c.makeRequest(ctx, messages, 0.1, false)
	if err != nil {
		return false, err
	}

	fmt.Printf("Supervisor Response: %s\n", resp)

	return strings.Contains(strings.ToUpper(resp), "ДА"), nil
}

// --- Helper ---

func (c *client) makeRequest(ctx context.Context, messages []chatMessage, temp float64, jsonMode bool) (string, error) {
	reqBody := chatRequest{
		Model:       "deepseek-chat", // Or "deepseek-coder" depending on availability
		Messages:    messages,
		Temperature: temp,
	}
	if jsonMode {
		reqBody.Format = &jsonFormat{Type: "json_object"}
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", deepSeekAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from AI")
	}

	return chatResp.Choices[0].Message.Content, nil
}
