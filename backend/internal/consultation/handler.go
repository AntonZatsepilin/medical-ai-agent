package consultation

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

type AudioInputRequest struct {
	ConsultationID string `json:"consultation_id"`
	Text           string `json:"text"` 
}

type CreateConsultationRequest struct {
	PatientID string `json:"patient_id"`
}

func (h *Handler) CreateConsultation(w http.ResponseWriter, r *http.Request) {
	var req CreateConsultationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	pid, err := uuid.Parse(req.PatientID)
	if err != nil {
		// For demo purposes, generate a new UUID if invalid/empty
		pid = uuid.New()
	}

	c, err := h.svc.CreateConsultation(r.Context(), pid)
	if err != nil {
		http.Error(w, "Failed to create consultation", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"consultation_id": c.ID.String(),
	})
}

func (h *Handler) HandleVoiceInput(w http.ResponseWriter, r *http.Request) {
	var req AudioInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(req.ConsultationID)
	if err != nil {
		http.Error(w, "Invalid consultation ID", http.StatusBadRequest)
		return
	}
	
	response, err := h.svc.ProcessUserAudio(r.Context(), id, req.Text)
	if err != nil {
		http.Error(w, "Processing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"response": response,
	})
}

type TTSRequest struct {
	Text string `json:"text"`
}

func (h *Handler) HandleTTS(w http.ResponseWriter, r *http.Request) {
	var req TTSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	audioData, err := h.svc.SynthesizeSpeech(r.Context(), req.Text)
	if err != nil {
		http.Error(w, "TTS failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Write(audioData)
}

func (h *Handler) HandleAudioUpload(w http.ResponseWriter, r *http.Request) {
	// Limit upload size (e.g. 10MB)
	r.ParseMultipartForm(10 << 20)

	consultationIDStr := r.FormValue("consultation_id")
	if consultationIDStr == "" {
		http.Error(w, "Missing consultation_id", http.StatusBadRequest)
		return
	}

	id, err := uuid.Parse(consultationIDStr)
	if err != nil {
		http.Error(w, "Invalid consultation ID", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("audio")
	if err != nil {
		http.Error(w, "Error retrieving audio file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read audio bytes
	// In a real app, we might stream this, but for now read into memory
	// or pass the reader if the client supports it. 
	// Our STTClient.Transcribe takes []byte currently.
	// Let's read it.
	// Note: For large files, this is bad practice, but for voice commands it's fine.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		http.Error(w, "Failed to read audio file", http.StatusInternalServerError)
		return
	}

	// 1. Transcribe
	text, err := h.svc.TranscribeAudio(r.Context(), buf.Bytes())
	if err != nil {
		http.Error(w, "Transcription failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if text == "" {
		// If silence or no speech detected
		json.NewEncoder(w).Encode(map[string]string{
			"response": "", 
			"text": "",
		})
		return
	}

	// 2. Process as if it was text input
	response, err := h.svc.ProcessUserAudio(r.Context(), id, text)
	if err != nil {
		http.Error(w, "Processing failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"response": response,
		"text":     text, // Return recognized text for UI
	})
}

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/consultation", h.CreateConsultation)
	r.Post("/consultation/chat", h.HandleVoiceInput)
	r.Post("/consultation/audio", h.HandleAudioUpload)
	r.Post("/tts", h.HandleTTS)
}
