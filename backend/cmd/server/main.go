package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"

	"medical-ai-agent/internal/agent"
	"medical-ai-agent/internal/consultation"
	"medical-ai-agent/internal/platform/telegram"
	"medical-ai-agent/internal/report"
	"strconv"
)

func main() {
	// 1. Infrastructure
	dbConnStr := os.Getenv("DATABASE_URL")
	if dbConnStr == "" {
		dbConnStr = "postgres://user:password@localhost:5432/medical_ai?sslmode=disable"
	}
	
	var db *sql.DB
	var err error
	
	// Simple retry logic for DB connection
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", dbConnStr)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		fmt.Printf("Waiting for DB... (%d/10)\n", i+1)
		// In a real app, use time.Sleep
	}
	if err != nil {
		log.Printf("Could not connect to DB: %v. Continuing without DB for demo purposes (some features will fail).\n", err)
	} else {
		log.Println("Connected to Database.")
	}

	// 2. Clients
	deepSeekKey := os.Getenv("DEEPSEEK_API_KEY")
	aiClient := agent.NewDeepSeekClient(deepSeekKey)

	// Use local Silero TTS
	ttsClient := agent.NewSileroClient()
	// Use local Whisper STT
	sttClient := agent.NewWhisperClient()

	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	tgClient := telegram.NewClient(tgToken)

	// 3. Services
	repo := consultation.NewRepository(db)
	
	// Run Migrations
	if dbConnStr != "" {
		m, err := migrate.New(
			"file://migrations",
			dbConnStr,
		)
		if err != nil {
			log.Printf("Migration init failed: %v", err)
		} else {
			if err := m.Up(); err != nil && err != migrate.ErrNoChange {
				log.Printf("Migration up failed: %v", err)
			} else {
				log.Println("Migrations applied successfully!")
			}
		}
	}

	doctorChatIDStr := os.Getenv("DOCTOR_CHAT_ID")
	doctorChatID, _ := strconv.ParseInt(doctorChatIDStr, 10, 64)
	if doctorChatID == 0 {
		log.Println("Warning: DOCTOR_CHAT_ID is not set or invalid. Reports will not be sent correctly.")
	}

	reportSvc := report.NewService(tgClient, doctorChatID)
	consultationSvc := consultation.NewService(repo, aiClient, ttsClient, sttClient, reportSvc)
	consultationHandler := consultation.NewHandler(consultationSvc)

	// 4. Router
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	
	// CORS for frontend
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			if r.Method == "OPTIONS" {
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.Route("/api", func(r chi.Router) {
		consultation.RegisterRoutes(r, consultationHandler)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}