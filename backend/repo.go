package main

import (
    "context"
    "errors"
    "strconv"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/jackc/pgx/v5/pgconn"
)

// Repository abstracts DB for easy testing
type Repository interface {
    CreatePrescription(ctx context.Context, p *Prescription) (*Prescription, error)
    TopDrugs(ctx context.Context, from, to time.Time, limit int, patientID *int64) ([]TopDrug, error)
    IsPhysicianPatientLinked(ctx context.Context, physicianID, patientID int64) (bool, error)
    ListPrescriptions(ctx context.Context, filter ListPrescriptionsFilter) ([]Prescription, error)
    // ListPatientsForPhysician returns patients linked to a physician (for dropdowns)
    ListPatientsForPhysician(ctx context.Context, physicianID int64) ([]Patient, error)
    // FindOrCreateDrug returns the id for a drug by name, inserting if it doesn't exist
    FindOrCreateDrug(ctx context.Context, name string) (int64, error)
    // ListPhysiciansForPatient returns physicians linked to a patient
    ListPhysiciansForPatient(ctx context.Context, patientID int64) ([]Physician, error)
}

// Sentinel errors for handler mapping
var (
    // ErrInvalidReference means a foreign key failed (patient_id, physician_id, or drug_id not found)
    ErrInvalidReference = errors.New("invalid reference")
)

// Postgres implementation
type PGRepo struct{ pool *pgxpool.Pool }

func NewPGRepo(ctx context.Context, dsn string) (*PGRepo, error) {
    pool, err := pgxpool.New(ctx, dsn)
    if err != nil {
        return nil, err
    }
    return &PGRepo{pool: pool}, nil
}

func (r *PGRepo) CreatePrescription(ctx context.Context, p *Prescription) (*Prescription, error) {
    // Do not pass prescribed_at from the application layer. Rely on the DB default (NOW()).
    // Passing Go's zero time results in year 0001 timestamps, which caused UI discrepancies.
    const q = `
        INSERT INTO prescriptions (patient_id, physician_id, drug_id, quantity, sig)
        VALUES ($1,$2,$3,$4,$5)
        RETURNING id, prescribed_at
    `
    row := r.pool.QueryRow(ctx, q, p.PatientID, p.PhysicianID, p.DrugID, p.Quantity, p.Sig)
    if err := row.Scan(&p.ID, &p.PrescribedAt); err != nil {
        // Translate common FK errors to a friendlier error the handler can map to 400
        var pgErr *pgconn.PgError
        if errors.As(err, &pgErr) {
            if pgErr.Code == "23503" { // foreign_key_violation
                return nil, ErrInvalidReference
            }
        }
        return nil, err
    }
    return p, nil
}

func (r *PGRepo) TopDrugs(ctx context.Context, from, to time.Time, limit int, patientID *int64) ([]TopDrug, error) {
    // Aggregate by total quantity for performance and usefulness
    base := `
        SELECT d.id, d.name, COALESCE(SUM(pr.quantity),0) AS total_qty
        FROM prescriptions pr
        JOIN drugs d ON d.id = pr.drug_id
        WHERE pr.prescribed_at >= $1 AND pr.prescribed_at < $2
    `
    args := []any{from, to}
    if patientID != nil {
        base += " AND pr.patient_id = $3"
        args = append(args, *patientID)
    }
    base += " GROUP BY d.id, d.name ORDER BY total_qty DESC, d.id ASC LIMIT " + strconv.Itoa(limit)

    rows, err := r.pool.Query(ctx, base, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []TopDrug
    for rows.Next() {
        var td TopDrug
        if err := rows.Scan(&td.DrugID, &td.DrugName, &td.TotalQty); err != nil {
            return nil, err
        }
        out = append(out, td)
    }
    return out, rows.Err()
}

func (r *PGRepo) IsPhysicianPatientLinked(ctx context.Context, physicianID, patientID int64) (bool, error) {
    const q = `SELECT 1 FROM physician_patients WHERE physician_id=$1 AND patient_id=$2 LIMIT 1`
    row := r.pool.QueryRow(ctx, q, physicianID, patientID)
    var one int
    if err := row.Scan(&one); err != nil {
        return false, nil
    }
    return true, nil
}

func (r *PGRepo) ListPatientsForPhysician(ctx context.Context, physicianID int64) ([]Patient, error) {
    const q = `
        SELECT p.id, p.name
        FROM physician_patients pp
        JOIN patients p ON p.id = pp.patient_id
        WHERE pp.physician_id = $1
        ORDER BY p.name ASC, p.id ASC
    `
    rows, err := r.pool.Query(ctx, q, physicianID)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Patient
    for rows.Next() {
        var it Patient
        if err := rows.Scan(&it.ID, &it.Name); err != nil { return nil, err }
        out = append(out, it)
    }
    return out, rows.Err()
}

func (r *PGRepo) FindOrCreateDrug(ctx context.Context, name string) (int64, error) {
    // Use UPSERT to return existing id when name already present
    const q = `
        INSERT INTO drugs(name)
        VALUES ($1)
        ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
        RETURNING id
    `
    var id int64
    if err := r.pool.QueryRow(ctx, q, name).Scan(&id); err != nil {
        return 0, err
    }
    return id, nil
}

func (r *PGRepo) ListPhysiciansForPatient(ctx context.Context, patientID int64) ([]Physician, error) {
    const q = `
        SELECT ph.id, ph.name
        FROM physician_patients pp
        JOIN physicians ph ON ph.id = pp.physician_id
        WHERE pp.patient_id = $1
        ORDER BY ph.name ASC, ph.id ASC
    `
    rows, err := r.pool.Query(ctx, q, patientID)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Physician
    for rows.Next() {
        var it Physician
        if err := rows.Scan(&it.ID, &it.Name); err != nil { return nil, err }
        out = append(out, it)
    }
    return out, rows.Err()
}

// ListPrescriptions returns prescriptions based on RBAC-aware filters
type ListPrescriptionsFilter struct {
    // Exactly one of PatientID or PhysicianID should typically be set based on caller role
    PatientID   *int64
    PhysicianID *int64
    Limit       int
}

func (r *PGRepo) ListPrescriptions(ctx context.Context, filter ListPrescriptionsFilter) ([]Prescription, error) {
    limit := filter.Limit
    if limit <= 0 || limit > 200 {
        limit = 50
    }
    q := `
        SELECT pr.id,
               pr.patient_id, p.name AS patient_name,
               pr.physician_id, ph.name AS physician_name,
               pr.drug_id, d.name AS drug_name,
               pr.quantity, pr.sig, pr.prescribed_at
        FROM prescriptions pr
        JOIN patients p   ON p.id = pr.patient_id
        JOIN physicians ph ON ph.id = pr.physician_id
        JOIN drugs d      ON d.id = pr.drug_id
        WHERE 1=1`
    args := []any{}
    if filter.PatientID != nil {
        q += " AND pr.patient_id = $" + strconv.Itoa(len(args)+1)
        args = append(args, *filter.PatientID)
    }
    if filter.PhysicianID != nil {
        q += " AND pr.physician_id = $" + strconv.Itoa(len(args)+1)
        args = append(args, *filter.PhysicianID)
    }
    q += " ORDER BY pr.prescribed_at DESC, pr.id DESC LIMIT " + strconv.Itoa(limit)

    rows, err := r.pool.Query(ctx, q, args...)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Prescription
    for rows.Next() {
        var p Prescription
        if err := rows.Scan(
            &p.ID,
            &p.PatientID, &p.PatientName,
            &p.PhysicianID, &p.PhysicianName,
            &p.DrugID, &p.DrugName,
            &p.Quantity, &p.Sig, &p.PrescribedAt,
        ); err != nil {
            return nil, err
        }
        out = append(out, p)
    }
    return out, rows.Err()
}

// noopRepo is a placeholder when no DB is configured
type noopRepo struct{}

func (n *noopRepo) CreatePrescription(ctx context.Context, p *Prescription) (*Prescription, error) {
    return nil, errors.New("db not configured")
}
func (n *noopRepo) TopDrugs(ctx context.Context, from, to time.Time, limit int, patientID *int64) ([]TopDrug, error) {
    return []TopDrug{}, nil
}
func (n *noopRepo) IsPhysicianPatientLinked(ctx context.Context, physicianID, patientID int64) (bool, error) {
    return false, nil
}
func (n *noopRepo) ListPrescriptions(ctx context.Context, filter ListPrescriptionsFilter) ([]Prescription, error) {
    return []Prescription{}, nil
}
func (n *noopRepo) ListPatientsForPhysician(ctx context.Context, physicianID int64) ([]Patient, error) {
    return []Patient{}, nil
}
func (n *noopRepo) FindOrCreateDrug(ctx context.Context, name string) (int64, error) {
    return 0, errors.New("db not configured")
}
func (n *noopRepo) ListPhysiciansForPatient(ctx context.Context, patientID int64) ([]Physician, error) {
    return []Physician{}, nil
}
