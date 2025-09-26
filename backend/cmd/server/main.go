package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"medical-ai-agent/internal/agent"
	"medical-ai-agent/internal/consultation"
	"medical-ai-agent/internal/platform/telegram"
	"medical-ai-agent/internal/report"
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

	tgToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	tgClient := telegram.NewClient(tgToken)

	// 3. Services
	repo := consultation.NewRepository(db)
	
	// Initialize DB Schema
	if db != nil {
		if err := repo.InitDB(context.Background()); err != nil {
			log.Printf("Failed to initialize DB schema: %v", err)
		}
	}

	reportSvc := report.NewService(tgClient, 123456789) // Doctor Chat ID
	consultationSvc := consultation.NewService(repo, aiClient, reportSvc)
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