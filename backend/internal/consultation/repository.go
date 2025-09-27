package consultation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Consultation, error)
	Save(ctx context.Context, c *Consultation) error
}

type postgresRepo struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepo{db: db}
}

func (r *postgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*Consultation, error) {
	query := `SELECT id, patient_id, history, facts, mood, is_complete, created_at, updated_at FROM consultations WHERE id = $1`
	
	row := r.db.QueryRowContext(ctx, query, id)
	
	var c Consultation
	var historyJSON, factsJSON []byte
	
	err := row.Scan(
		&c.ID,
		&c.PatientID,
		&historyJSON,
		&factsJSON,
		&c.CurrentMood,
		&c.IsComplete,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("consultation not found")
		}
		return nil, err
	}

	if len(historyJSON) > 0 {
		if err := json.Unmarshal(historyJSON, &c.History); err != nil {
			return nil, fmt.Errorf("failed to unmarshal history: %w", err)
		}
	}
	if len(factsJSON) > 0 {
		if err := json.Unmarshal(factsJSON, &c.ExtractedFacts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal facts: %w", err)
		}
	}

	return &c, nil
}

func (r *postgresRepo) Save(ctx context.Context, c *Consultation) error {
	historyJSON, err := json.Marshal(c.History)
	if err != nil {
		return err
	}
	factsJSON, err := json.Marshal(c.ExtractedFacts)
	if err != nil {
		return err
	}

	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.UpdatedAt = time.Now()

	query := `
		INSERT INTO consultations (id, patient_id, history, facts, mood, is_complete, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			history = $3,
			facts = $4,
			mood = $5,
			is_complete = $6,
			updated_at = $8
	`
	_, err = r.db.ExecContext(ctx, query, 
		c.ID, c.PatientID, historyJSON, factsJSON, c.CurrentMood, c.IsComplete, c.CreatedAt, c.UpdatedAt)
	return err
}
