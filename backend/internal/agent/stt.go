package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Local STT Service URL (same as TTS service, different endpoint)
const sttServiceURL = "http://tts:8000/transcribe"

type STTClient interface {
	Transcribe(ctx context.Context, audioData []byte) (string, error)
}

type whisperClient struct {
	httpClient *http.Client
}

func NewWhisperClient() STTClient {
	return &whisperClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type sttResponse struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}

func (c *whisperClient) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Create form file
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	
	if _, err := part.Write(audioData); err != nil {
		return "", err
	}
	
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sttServiceURL, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("STT API error: %s - %s", resp.Status, string(respBody))
	}

	var result sttResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Text, nil
}
