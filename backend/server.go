package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "strconv"
    "time"
    "os"
)

type Server struct {
    repo Repository
    mux  *http.ServeMux
    allowOrigin string
}

func NewServer(repo Repository) *Server {
    s := &Server{repo: repo, mux: http.NewServeMux()}
    // Allow CORS from configured web origin (e.g., http://localhost:5173)
    if v := os.Getenv("WEB_ORIGIN"); v != "" {
        s.allowOrigin = v
    } else {
        // Provide a sensible default for local dev if not configured
        s.allowOrigin = "http://localhost:5173"
    }
    s.routes()
    return s
}

func (s *Server) routes() {
    s.mux.HandleFunc("/prescriptions", s.handlePrescriptions)
    s.mux.HandleFunc("/analytics/top-drugs", s.handleTopDrugs)
    s.mux.HandleFunc("/physicians/", s.handlePhysicianSubroutes)
    s.mux.HandleFunc("/patients/", s.handlePatientSubroutes)
    // Readiness endpoint that also checks DB connectivity when possible
    s.mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            w.Header().Set("Allow", http.MethodGet)
            writeError(w, http.StatusMethodNotAllowed, "method not allowed")
            return
        }
        // Default payload
        status := map[string]any{"status": "ok", "db": "unknown"}
        if pg, ok := s.repo.(*PGRepo); ok {
            ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
            defer cancel()
            // lightweight ping
            if err := pg.pool.Ping(ctx); err != nil {
                status["db"] = "down"
                writeJSON(w, http.StatusServiceUnavailable, status)
                return
            }
            status["db"] = "ok"
        }
        writeJSON(w, http.StatusOK, status)
    })
    // Simple health endpoint for readiness/liveness checks
    s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            w.Header().Set("Allow", http.MethodGet)
            writeError(w, http.StatusMethodNotAllowed, "method not allowed")
            return
        }
        writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
    })
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Minimal CORS
    if s.allowOrigin != "" {
        origin := r.Header.Get("Origin")
        // Support multiple origins via comma-separated WEB_ORIGIN, or wildcard "*"
        ao := s.allowOrigin
        if ao == "*" {
            w.Header().Set("Access-Control-Allow-Origin", "*")
        } else if origin != "" {
            // pick matching origin from list if provided
            matched := false
            for _, candidate := range splitCSV(ao) {
                if candidate == origin {
                    w.Header().Set("Access-Control-Allow-Origin", origin)
                    matched = true
                    break
                }
            }
            if !matched {
                // fall back to configured single origin if no match
                w.Header().Set("Access-Control-Allow-Origin", ao)
            }
        } else {
            w.Header().Set("Access-Control-Allow-Origin", ao)
        }
        w.Header().Set("Vary", "Origin")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Role, X-User-ID")
        w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
    }
    if r.Method == http.MethodOptions {
        w.WriteHeader(http.StatusNoContent)
        return
    }
    s.mux.ServeHTTP(w, r)
}

// splitCSV splits a comma-separated list, trimming spaces and ignoring empties.
func splitCSV(s string) []string {
    var out []string
    start := 0
    for i := 0; i <= len(s); i++ {
        if i == len(s) || s[i] == ',' {
            // trim spaces
            part := s[start:i]
            // trim leading
            for len(part) > 0 && (part[0] == ' ' || part[0] == '\t') { part = part[1:] }
            // trim trailing
            for len(part) > 0 && (part[len(part)-1] == ' ' || part[len(part)-1] == '\t') { part = part[:len(part)-1] }
            if part != "" { out = append(out, part) }
            start = i + 1
        }
    }
    return out
}

type createPrescriptionReq struct {
    PatientID   int64  `json:"patient_id"`
    PhysicianID int64  `json:"physician_id"`
    DrugID      int64  `json:"drug_id"`
    DrugName    string `json:"drug_name"`
    Quantity    int    `json:"quantity"`
    Sig         string `json:"sig"`
}

func (req *createPrescriptionReq) validate() error {
    if req.PatientID <= 0 { return fmt.Errorf("patient_id must be > 0") }
    if req.PhysicianID <= 0 { return fmt.Errorf("physician_id must be > 0") }
    // Either a positive drug_id or a non-empty drug_name must be provided
    if req.DrugID <= 0 {
        if len(req.DrugName) == 0 {
            return fmt.Errorf("either drug_id (>0) or drug_name is required")
        }
        if len(req.DrugName) > 200 {
            return fmt.Errorf("drug_name too long")
        }
    }
    if req.Quantity <= 0 { return fmt.Errorf("quantity must be > 0") }
    if len(req.Sig) == 0 { return fmt.Errorf("sig is required") }
    if len(req.Sig) > 500 { return fmt.Errorf("sig too long") }
    return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handlePrescriptions(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodGet {
        s.handleListPrescriptions(w, r)
        return
    }
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", http.MethodPost+", "+http.MethodGet)
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    role, err := readRole(r)
    if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
    // Only physicians may create prescriptions; admins and patients are forbidden
    if role != RolePhysician { writeError(w, http.StatusForbidden, "only physicians may create prescriptions"); return }

    // Read caller id only if needed (physician/patient flows)
    // Caller must be the physician creating the prescription
    callerID, err := readUserID(r)
    if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }

    var req createPrescriptionReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON body")
        return
    }
    if err := req.validate(); err != nil {
        writeError(w, http.StatusBadRequest, err.Error())
        return
    }

    // RBAC checks
    // Physician RBAC: must create as themselves and be linked to patient
    if req.PhysicianID != callerID {
        writeError(w, http.StatusForbidden, "physicians may only create as themselves")
        return
    }
    linked, err := s.repo.IsPhysicianPatientLinked(r.Context(), callerID, req.PatientID)
    if err != nil { writeError(w, http.StatusInternalServerError, "link check failed"); return }
    if !linked { writeError(w, http.StatusForbidden, "physician not linked to patient"); return }

    // Resolve drug id: use provided id, or find/create by name
    var drugID int64 = req.DrugID
    if drugID <= 0 {
        name := req.DrugName
        if name == "" { writeError(w, http.StatusBadRequest, "drug_name is required when drug_id is not provided"); return }
        // Normalize: trim spaces
        for len(name) > 0 && (name[0] == ' ' || name[0] == '\t') { name = name[1:] }
        for len(name) > 0 && (name[len(name)-1] == ' ' || name[len(name)-1] == '\t') { name = name[:len(name)-1] }
        if name == "" { writeError(w, http.StatusBadRequest, "drug_name cannot be blank"); return }
        id, err := s.repo.FindOrCreateDrug(r.Context(), name)
        if err != nil { writeError(w, http.StatusInternalServerError, "failed to resolve drug"); return }
        drugID = id
    }

    p := &Prescription{
        PatientID: req.PatientID, PhysicianID: req.PhysicianID, DrugID: drugID,
        Quantity: req.Quantity, Sig: req.Sig,
    }
    created, err := s.repo.CreatePrescription(r.Context(), p)
    if err != nil {
        if errors.Is(err, ErrInvalidReference) {
            writeError(w, http.StatusBadRequest, "invalid patient_id, physician_id, or drug_id")
            return
        }
        writeError(w, http.StatusInternalServerError, "failed to create prescription")
        return
    }
    writeJSON(w, http.StatusCreated, created)
}

// handleListPrescriptions returns prescriptions according to RBAC
func (s *Server) handleListPrescriptions(w http.ResponseWriter, r *http.Request) {
    role, err := readRole(r)
    if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
    limit := 50
    if ls := r.URL.Query().Get("limit"); ls != "" {
        if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 200 { limit = n } else {
            writeError(w, http.StatusBadRequest, "limit must be 1..200"); return
        }
    }
    var filter ListPrescriptionsFilter
    filter.Limit = limit
    switch role {
    case RolePatient:
        id, err := readUserID(r); if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
        filter.PatientID = &id
    case RolePhysician:
        id, err := readUserID(r); if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
        filter.PhysicianID = &id
    case RoleAdmin:
        // Optional filters for admin via query params
        if v := r.URL.Query().Get("patient_id"); v != "" {
            if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 { filter.PatientID = &n } else { writeError(w, http.StatusBadRequest, "invalid patient_id"); return }
        }
        if v := r.URL.Query().Get("physician_id"); v != "" {
            if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 { filter.PhysicianID = &n } else { writeError(w, http.StatusBadRequest, "invalid physician_id"); return }
        }
    }
    items, err := s.repo.ListPrescriptions(r.Context(), filter)
    if err != nil { writeError(w, http.StatusInternalServerError, "failed to list prescriptions"); return }
    writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit})
}

// handlePhysicianSubroutes handles endpoints under /physicians/{id}/...
func (s *Server) handlePhysicianSubroutes(w http.ResponseWriter, r *http.Request) {
    // Expected path: /physicians/{id}/patients
    // Basic parse
    // Trim prefix
    path := r.URL.Path
    if len(path) < len("/physicians/") || path[:len("/physicians/")] != "/physicians/" {
        writeError(w, http.StatusNotFound, "not found")
        return
    }
    rest := path[len("/physicians/"):]
    // find next '/'
    slash := -1
    for i := 0; i < len(rest); i++ { if rest[i] == '/' { slash = i; break } }
    if slash == -1 {
        writeError(w, http.StatusNotFound, "not found")
        return
    }
    idStr := rest[:slash]
    tail := rest[slash:]
    // currently only /patients
    if tail != "/patients" {
        writeError(w, http.StatusNotFound, "not found")
        return
    }
    // RBAC
    role, err := readRole(r)
    if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil || id <= 0 { writeError(w, http.StatusBadRequest, "invalid physician id in path"); return }

    switch role {
    case RolePatient:
        writeError(w, http.StatusForbidden, "patients cannot access this resource")
        return
    case RolePhysician:
        callerID, err := readUserID(r)
        if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
        if callerID != id {
            writeError(w, http.StatusForbidden, "physicians may only view their own patients")
            return
        }
    case RoleAdmin:
        // allowed
    }

    items, err := s.repo.ListPatientsForPhysician(r.Context(), id)
    if err != nil { writeError(w, http.StatusInternalServerError, "failed to list patients"); return }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// handlePatientSubroutes handles endpoints under /patients/{id}/...
func (s *Server) handlePatientSubroutes(w http.ResponseWriter, r *http.Request) {
    // Expected path: /patients/{id}/physicians
    path := r.URL.Path
    if len(path) < len("/patients/") || path[:len("/patients/")] != "/patients/" {
        writeError(w, http.StatusNotFound, "not found")
        return
    }
    rest := path[len("/patients/"):]
    slash := -1
    for i := 0; i < len(rest); i++ { if rest[i] == '/' { slash = i; break } }
    if slash == -1 { writeError(w, http.StatusNotFound, "not found"); return }
    idStr := rest[:slash]
    tail := rest[slash:]
    if tail != "/physicians" { writeError(w, http.StatusNotFound, "not found"); return }

    // RBAC: patients can only view their own physicians; admin allowed; physicians forbidden
    role, err := readRole(r)
    if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil || id <= 0 { writeError(w, http.StatusBadRequest, "invalid patient id in path"); return }

    switch role {
    case RolePhysician:
        writeError(w, http.StatusForbidden, "physicians cannot access this resource")
        return
    case RolePatient:
        callerID, err := readUserID(r)
        if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
        if callerID != id { writeError(w, http.StatusForbidden, "patients may only view their own physicians"); return }
    case RoleAdmin:
        // allowed
    }

    items, err := s.repo.ListPhysiciansForPatient(r.Context(), id)
    if err != nil { writeError(w, http.StatusInternalServerError, "failed to list physicians"); return }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleTopDrugs(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.Header().Set("Allow", http.MethodGet)
        writeError(w, http.StatusMethodNotAllowed, "method not allowed")
        return
    }
    role, err := readRole(r)
    if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }

    q := r.URL.Query()
    fromS, toS := q.Get("from"), q.Get("to")
    if fromS == "" || toS == "" {
        writeError(w, http.StatusBadRequest, "from and to query params are required (RFC3339 date or datetime)")
        return
    }
    from, err1 := time.Parse(time.RFC3339, fromS)
    to, err2 := time.Parse(time.RFC3339, toS)
    if err1 != nil || err2 != nil || !to.After(from) {
        writeError(w, http.StatusBadRequest, "invalid from/to range")
        return
    }
    limit := 10
    if ls := q.Get("limit"); ls != "" {
        if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 100 {
            limit = n
        } else {
            writeError(w, http.StatusBadRequest, "limit must be 1..100")
            return
        }
    }

    var patientID *int64
    if role == RolePatient {
        id, err := readUserID(r)
        if err != nil { writeError(w, http.StatusUnauthorized, err.Error()); return }
        patientID = &id
    }

    results, err := s.repo.TopDrugs(r.Context(), from, to, limit, patientID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to fetch analytics")
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{
        "from": from, "to": to, "limit": limit, "items": results,
    })
}
