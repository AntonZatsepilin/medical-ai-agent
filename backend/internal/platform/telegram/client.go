package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
			Timeout: 30 * time.Second, // Increased timeout for file uploads
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

func (c *Client) SendDocument(chatID int64, fileData []byte, fileName string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", c.Token)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add chat_id field
	if err := writer.WriteField("chat_id", fmt.Sprintf("%d", chatID)); err != nil {
		return err
	}

	// Add file field
	part, err := writer.CreateFormFile("document", fileName)
	if err != nil {
		return err
	}
	if _, err := part.Write(fileData); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var bodyBytes []byte
		if resp.Body != nil {
			bodyBytes, _ = io.ReadAll(resp.Body)
		}
		return fmt.Errorf("telegram api returned status: %s, body: %s", resp.Status, string(bodyBytes))
	}

	return nil
}
