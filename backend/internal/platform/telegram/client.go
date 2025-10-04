package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	Token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		Token: token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type sendMessageReq struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (c *Client) SendMessage(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.Token)

	reqBody := sendMessageReq{
		ChatID:    chatID,
		Text:      text,
		// ParseMode: "Markdown", // Disable Markdown to avoid parsing errors with special characters
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read body to see the error message from Telegram
		var bodyBytes []byte
		if resp.Body != nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("telegram api returned status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	return nil
}
