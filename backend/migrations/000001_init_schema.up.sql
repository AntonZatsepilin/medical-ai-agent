CREATE TABLE IF NOT EXISTS consultations (
    id UUID PRIMARY KEY,
    patient_id UUID NOT NULL,
    history JSONB,
    facts JSONB,
    mood TEXT,
    is_complete BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_consultations_patient_id ON consultations(patient_id);
