package report

import (
	"bytes"
	"context"
	"fmt"
	"medical-ai-agent/internal/consultation"
	"time"

	"github.com/signintech/gopdf"
)

type TelegramClient interface {
	SendMessage(chatID int64, text string) error
	SendDocument(chatID int64, fileData []byte, fileName string) error
}

type Service struct {
	tgClient     TelegramClient
	doctorChatID int64
}

func NewService(tg TelegramClient, doctorChatID int64) *Service {
	return &Service{
		tgClient:     tg,
		doctorChatID: doctorChatID,
	}
}

func (s *Service) SendDoctorReport(ctx context.Context, c consultation.Consultation) error {
	fmt.Printf("Generating PDF report for consultation %s...\n", c.ID)
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	// Load Font (DejaVuSans supports Cyrillic)
	// Path inside Docker container (alpine with ttf-dejavu installed)
	fontPath := "/usr/share/fonts/ttf-dejavu/DejaVuSans.ttf"
	err := pdf.AddTTFFont("DejaVu", fontPath)
	if err != nil {
		fmt.Printf("Error loading font: %v\n", err)
		return fmt.Errorf("failed to load font for PDF (path: %s): %w", fontPath, err)
	}

	if err := pdf.SetFont("DejaVu", "", 20); err != nil {
		return err
	}

	// Header
	pdf.Cell(nil, "Медицинский отчет (AI Agent)")
	pdf.Br(30)

	// Patient Info
	if err := pdf.SetFont("DejaVu", "", 12); err != nil { return err }
	pdf.Cell(nil, fmt.Sprintf("Дата: %s", time.Now().Format("02.01.2006 15:04")))
	pdf.Br(15)
	pdf.Cell(nil, fmt.Sprintf("ID Пациента: %s", c.PatientID))
	pdf.Br(15)
	pdf.Cell(nil, fmt.Sprintf("Эмоциональное состояние: %s", translateMood(c.CurrentMood)))
	pdf.Br(25)

	// Facts
	if err := pdf.SetFont("DejaVu", "", 14); err != nil { return err }
	pdf.Cell(nil, "Собранные факты:")
	pdf.Br(15)

	if err := pdf.SetFont("DejaVu", "", 11); err != nil { return err }
	if len(c.ExtractedFacts) == 0 {
		pdf.Cell(nil, "- Факты не выявлены.")
		pdf.Br(15)
	}
	for _, fact := range c.ExtractedFacts {
		line := fmt.Sprintf("- [%s] %s (Уверенность: %s)", fact.Category, fact.Description, fact.Confidence)
		lines, _ := pdf.SplitText(line, 500)
		for _, l := range lines {
			pdf.Cell(nil, l)
			pdf.Br(12)
		}
		pdf.Br(5)
	}
	pdf.Br(15)

	// Recommendations
	if c.Recommendations != "" {
		if err := pdf.SetFont("DejaVu", "", 14); err != nil { return err }
		pdf.Cell(nil, "Рекомендации и Анализ:")
		pdf.Br(15)
		if err := pdf.SetFont("DejaVu", "", 11); err != nil { return err }
		
		lines, _ := pdf.SplitText(c.Recommendations, 500)
		for _, l := range lines {
			pdf.Cell(nil, l)
			pdf.Br(12)
		}
	}

	// Footer
	pdf.SetY(270)
	if err := pdf.SetFont("DejaVu", "", 9); err != nil { return err }
	pdf.Cell(nil, "Сгенерировано автоматически Medical AI Agent")

	// Write to buffer
	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return fmt.Errorf("failed to write PDF: %w", err)
	}

	fileName := fmt.Sprintf("report_%s.pdf", c.ID.String())
	fmt.Printf("Sending PDF document to Telegram chat %d...\n", s.doctorChatID)
	err = s.tgClient.SendDocument(s.doctorChatID, buf.Bytes(), fileName)
	if err != nil {
		fmt.Printf("Error sending Telegram document: %v\n", err)
		return err
	}
	fmt.Println("PDF report sent successfully.")
	return nil
}

func translateMood(mood consultation.EmotionalState) string {
	switch mood {
	case consultation.StateAnxious:
		return "Тревожное"
	case consultation.StateCritical:
		return "Критическое"
	case consultation.StateCalm:
		return "Спокойное"
	case consultation.StateNeutral:
		return "Нейтральное"
	default:
		return string(mood)
	}
}
