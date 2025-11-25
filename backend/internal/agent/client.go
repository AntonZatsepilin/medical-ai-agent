package agent

import (
	"bufio"
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
	RunCommunicatorStream(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (<-chan string, <-chan error)
	RunAnalyst(ctx context.Context, history []consultation.Message) ([]consultation.MedicalFact, error)
	RunSupervisor(ctx context.Context, history []consultation.Message, facts []consultation.MedicalFact) (bool, error)
	GenerateRecommendations(ctx context.Context, facts []consultation.MedicalFact) (string, error)
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
	Stream      bool          `json:"stream,omitempty"`
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
		Delta   chatMessage `json:"delta"`
	} `json:"choices"`
}

// --- Implementations ---

func (c *client) RunCommunicatorStream(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (<-chan string, <-chan error) {
	systemPrompt := fmt.Sprintf(`Ты — заботливый и чуткий медицинский ассистент в приемном отделении.
Твоя главная цель: успокоить пациента и мягко выяснить причину обращения, пока он ожидает врача.
Текущее настроение пациента (по твоей оценке): %s.

ПРИНЦИПЫ ОБЩЕНИЯ:
1. **Эмпатия и Теплота**: Используй фразы "Я понимаю, как это неприятно", "Мне очень жаль, что вам больно", "Мы обязательно вам поможем". Твой тон должен быть мягким, человечным, не роботизированным.
2. **Активное слушание**: Подтверждай, что ты услышал пациента (например, "Хорошо, значит боль в животе...").
3. **Поддержка**: Если пациент тревожится, обязательно успокой его перед тем, как задать следующий вопрос.

ИНСТРУКЦИЯ ПО ФОРМАТУ ОТВЕТА:
1. Сначала оцени настроение пациента: "Спокойное", "Тревожное", "Критическое".
2. Напиши ответ пациенту.
3. Формат вывода: "[MOOD: <настроение>] <Текст ответа>"

Пример: "[MOOD: Тревожное] Я вижу, что вы очень переживаете. Пожалуйста, постарайтесь дышать глубже, вы уже в больнице и в безопасности. Скажите, как давно началась эта боль?"

ВАЖНО:
- Не ставь диагнозы.
- Задавай только ОДИН вопрос за раз, чтобы не перегружать пациента.
- Если ты собрал достаточно информации (основные жалобы, длительность, характер боли) или пациент сказал, что больше жалоб нет, ОБЯЗАТЕЛЬНО заверши диалог фразой: "Спасибо, врач скоро подойдет". Это сигнал для системы отправить отчет.`, mood)

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	for _, msg := range history {
		messages = append(messages, chatMessage{Role: msg.Role, Content: msg.Content})
	}

	return c.makeStreamRequest(ctx, messages, 0.7)
}

func (c *client) makeStreamRequest(ctx context.Context, messages []chatMessage, temp float64) (<-chan string, <-chan error) {
	tokenChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(tokenChan)
		defer close(errChan)

		reqBody := chatRequest{
			Model:       "deepseek-chat",
			Messages:    messages,
			Temperature: temp,
			Stream:      true,
		}

		jsonBody, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(ctx, "POST", deepSeekAPIURL, bytes.NewBuffer(jsonBody))
		if err != nil {
			errChan <- err
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errChan <- fmt.Errorf("API error: %s - %s", resp.Status, string(body))
			return
		}

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					errChan <- err
				}
				return
			}

			lineStr := strings.TrimSpace(string(line))
			if !strings.HasPrefix(lineStr, "data: ") {
				continue
			}

			data := strings.TrimPrefix(lineStr, "data: ")
			if data == "[DONE]" {
				return
			}

			var chatResp chatResponse
			if err := json.Unmarshal([]byte(data), &chatResp); err != nil {
				continue
			}

			if len(chatResp.Choices) > 0 {
				content := chatResp.Choices[0].Delta.Content
				if content != "" {
					tokenChan <- content
				}
			}
		}
	}()

	return tokenChan, errChan
}

func (c *client) RunCommunicator(ctx context.Context, history []consultation.Message, mood consultation.EmotionalState) (string, consultation.EmotionalState, error) {
	systemPrompt := fmt.Sprintf(`Ты — заботливый и чуткий медицинский ассистент в приемном отделении.
Твоя главная цель: успокоить пациента и мягко выяснить причину обращения, пока он ожидает врача.
Текущее настроение пациента (по твоей оценке): %s.

ПРИНЦИПЫ ОБЩЕНИЯ:
1. **Эмпатия и Теплота**: Используй фразы "Я понимаю, как это неприятно", "Мне очень жаль, что вам больно", "Мы обязательно вам поможем". Твой тон должен быть мягким, человечным, не роботизированным.
2. **Активное слушание**: Подтверждай, что ты услышал пациента (например, "Хорошо, значит боль в животе...").
3. **Поддержка**: Если пациент тревожится, обязательно успокой его перед тем, как задать следующий вопрос.

ИНСТРУКЦИЯ ПО ФОРМАТУ ОТВЕТА:
1. Сначала оцени настроение пациента: "Спокойное", "Тревожное", "Критическое".
2. Напиши ответ пациенту.
3. Формат вывода: "[MOOD: <настроение>] <Текст ответа>"

Пример: "[MOOD: Тревожное] Я вижу, что вы очень переживаете. Пожалуйста, постарайтесь дышать глубже, вы уже в больнице и в безопасности. Скажите, как давно началась эта боль?"

ВАЖНО:
- Не ставь диагнозы.
- Задавай только ОДИН вопрос за раз, чтобы не перегружать пациента.
- Если ты собрал достаточно информации (основные жалобы, длительность, характер боли) или пациент сказал, что больше жалоб нет, ОБЯЗАТЕЛЬНО заверши диалог фразой: "Спасибо, врач скоро подойдет". Это сигнал для системы отправить отчет.`, mood)

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
			case "спокойное", "calm", "neutral", "нейтральное":
				newMood = consultation.StateCalm
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

КРИТЕРИИ УВЕРЕННОСТИ:
- "Высокая": Пациент сказал четко и прямо (напр. "Болит голова 3 дня").
- "Средняя": Пациент выразился неточно или использовал слова "вроде", "наверное" (напр. "Кажется, температура была").
- "Низкая": Информацию пришлось додумывать или пациент путается в показаниях.

ВАЖНО:
- Анализируй каждое сообщение внимательно.
- Если пациент упоминает боль, обязательно фиксируй её характер, локализацию и длительность как отдельные факты или один подробный.
- Если пациент отрицает симптомы (напр. "температуры нет"), это тоже важный факт (category: "Отсутствие симптома").

Если новых фактов нет, верни пустой массив [].`

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	// Only analyze last few messages to save tokens and focus on recent context
	startIdx := 0
	if len(history) > 10 { // Increased context window for better analysis
		startIdx = len(history) - 10
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
	// Don't even bother the AI if we have very little history
	if len(history) < 4 { // Reduced minimum history check to allow quicker completion if needed
		return false, nil
	}

	factsSummary := ""
	for _, f := range facts {
		factsSummary += fmt.Sprintf("- %s: %s\n", f.Category, f.Description)
	}

	systemPrompt := fmt.Sprintf(`Ты — супервайзер медицинского опроса.
Собранные факты:
%s
Твоя задача — решить, можно ли ЗАВЕРШАТЬ опрос и отправлять отчет врачу.

КРИТЕРИИ ЗАВЕРШЕНИЯ (Достаточно выполнения ЛЮБОГО из пунктов):
1. Мы знаем основную жалобу пациента, её длительность и характер.
2. Пациент явно сказал "это всё", "больше ничего", "нет" на вопрос о других жалобах.
3. Собрано достаточно фактов для первичной сортировки (триажа).

Если пациент только поздоровался или мы знаем только "болит живот" без подробностей — отвечай "НЕТ".
Во всех остальных случаях, если картина ясна — отвечай "ДА".

Ответь ТОЛЬКО словом "ДА" или "НЕТ".`, factsSummary)

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}
	
	resp, err := c.makeRequest(ctx, messages, 0.1, false)
	if err != nil {
		return false, err
	}

	fmt.Printf("Supervisor Response: %s\n", resp)

	return strings.Contains(strings.ToUpper(resp), "ДА"), nil
}

func (c *client) GenerateRecommendations(ctx context.Context, facts []consultation.MedicalFact) (string, error) {
	factsSummary := ""
	for _, f := range facts {
		factsSummary += fmt.Sprintf("- %s: %s (Уверенность: %s)\n", f.Category, f.Description, f.Confidence)
	}

	systemPrompt := fmt.Sprintf(`Ты — старший врач-консультант.
На основе собранных фактов составь краткие рекомендации для дежурного врача.
Факты:
%s

Твоя задача:
1. Предположить возможную срочность (Триаж: Зеленый/Желтый/Красный).
2. Предложить список необходимых обследований (анализы, рентген и т.д.).
3. Дать краткое резюме случая.

Ответ должен быть кратким, структурированным текстом (не JSON).`, factsSummary)

	messages := []chatMessage{{Role: "system", Content: systemPrompt}}

	return c.makeRequest(ctx, messages, 0.3, false)
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
