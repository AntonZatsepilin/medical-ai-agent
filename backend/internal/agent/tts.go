package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Local Silero TTS Service URL (from docker-compose)
const ttsServiceURL = "http://tts:8000/generate"

type TTSClient interface {
	Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error)
}

type sileroClient struct {
	httpClient *http.Client
}

func NewSileroClient() TTSClient {
	return &sileroClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ttsRequest struct {
	Text    string `json:"text"`
	Speaker string `json:"speaker"` // xenia, kseniya, aidar, baya, eugene
}

func (c *sileroClient) Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error) {
	// Map "voiceID" to Silero speakers if needed, or use default
	speaker := "kseniya" // Default female voice
	if voiceID != "" {
		speaker = voiceID
	}

	reqBody := ttsRequest{
		Text:    text,
		Speaker: speaker,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", ttsServiceURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS API error: %s - %s", resp.Status, string(body))
	}

	return io.ReadAll(resp.Body)
}
