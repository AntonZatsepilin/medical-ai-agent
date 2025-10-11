package consultation

import (
	"encoding/json"
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

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/consultation", h.CreateConsultation)
	r.Post("/consultation/chat", h.HandleVoiceInput)
	r.Post("/tts", h.HandleTTS)
}
