package telegram

import (
	"fmt"
)

type Client struct {
	Token string
}

func NewClient(token string) *Client {
	return &Client{Token: token}
}

func (c *Client) SendMessage(chatID int64, text string) error {
	// In a real app, use http.Post to Telegram Bot API
	// https://api.telegram.org/bot<token>/sendMessage
	fmt.Printf("[Telegram] Sending to %d: %s\n", chatID, text)
	return nil
}
