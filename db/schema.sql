-- PostgreSQL schema for HealthCarePortal

CREATE TABLE IF NOT EXISTS patients (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS physicians (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS drugs (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS physician_patients (
    physician_id BIGINT NOT NULL REFERENCES physicians(id) ON DELETE CASCADE,
    patient_id   BIGINT NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    PRIMARY KEY (physician_id, patient_id)
);

CREATE TABLE IF NOT EXISTS prescriptions (
    id BIGSERIAL PRIMARY KEY,
    patient_id   BIGINT NOT NULL REFERENCES patients(id) ON DELETE RESTRICT,
    physician_id BIGINT NOT NULL REFERENCES physicians(id) ON DELETE RESTRICT,
    drug_id      BIGINT NOT NULL REFERENCES drugs(id) ON DELETE RESTRICT,
    quantity     INT    NOT NULL CHECK (quantity > 0),
    sig          TEXT   NOT NULL,
    prescribed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes to support analytics efficiently
CREATE INDEX IF NOT EXISTS idx_prescriptions_date ON prescriptions(prescribed_at);
CREATE INDEX IF NOT EXISTS idx_prescriptions_patient ON prescriptions(patient_id);
CREATE INDEX IF NOT EXISTS idx_prescriptions_drug ON prescriptions(drug_id);
CREATE INDEX IF NOT EXISTS idx_prescriptions_range_patient ON prescriptions(patient_id, prescribed_at);

-- Ensure natural key uniqueness for idempotent seeds
CREATE UNIQUE INDEX IF NOT EXISTS idx_patients_name ON patients(name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_physicians_name ON physicians(name);
