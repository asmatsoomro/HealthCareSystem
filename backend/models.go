package main

import "time"

// Domain models (minimal for handlers)
type Prescription struct {
    ID           int64     `json:"id"`
    PatientID    int64     `json:"patient_id"`
    PatientName  string    `json:"patient_name,omitempty"`
    PhysicianID  int64     `json:"physician_id"`
    PhysicianName string   `json:"physician_name,omitempty"`
    DrugID       int64     `json:"drug_id"`
    DrugName     string    `json:"drug_name,omitempty"`
    Quantity     int       `json:"quantity"`
    Sig          string    `json:"sig"`
    PrescribedAt time.Time `json:"prescribed_at"`
}

type TopDrug struct {
    DrugID   int64  `json:"drug_id"`
    DrugName string `json:"drug_name"`
    TotalQty int64  `json:"total_quantity"`
}

// Lightweight list item used for dropdowns
type Patient struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

// Lightweight physician item for patient-linked physician lists
type Physician struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}
