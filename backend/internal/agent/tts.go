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

const elevenLabsAPIURL = "https://api.elevenlabs.io/v1/text-to-speech"

type TTSClient interface {
	Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error)
}

type elevenLabsClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewElevenLabsClient(apiKey string) TTSClient {
	return &elevenLabsClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type ttsRequest struct {
	Text    string `json:"text"`
	ModelID string `json:"model_id"`
	VoiceSettings struct {
		Stability       float64 `json:"stability"`
		SimilarityBoost float64 `json:"similarity_boost"`
	} `json:"voice_settings"`
}

func (c *elevenLabsClient) Synthesize(ctx context.Context, text string, voiceID string) ([]byte, error) {
	if voiceID == "" {
		voiceID = "21m00Tcm4TlvDq8ikWAM" // Default Rachel
	}

	url := fmt.Sprintf("%s/%s", elevenLabsAPIURL, voiceID)

	reqBody := ttsRequest{
		Text:    text,
		ModelID: "eleven_multilingual_v2", // Best for Russian
	}
	// Default settings for stability
	reqBody.VoiceSettings.Stability = 0.5
	reqBody.VoiceSettings.SimilarityBoost = 0.75

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", c.apiKey)

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
