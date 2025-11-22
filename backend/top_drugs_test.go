package main

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

// fakeRepo implements Repository for tests
type fakeRepo struct {
    // configurable outputs
    top []TopDrug
    // capture inputs
    gotFrom, gotTo time.Time
    gotLimit       int
    gotPatientID   *int64
}

func (f *fakeRepo) CreatePrescription(_ context.Context, p *Prescription) (*Prescription, error) { return nil, nil }
func (f *fakeRepo) TopDrugs(_ context.Context, from, to time.Time, limit int, patientID *int64) ([]TopDrug, error) {
    f.gotFrom, f.gotTo, f.gotLimit, f.gotPatientID = from, to, limit, patientID
    return f.top, nil
}
func (f *fakeRepo) IsPhysicianPatientLinked(_ context.Context, physicianID, patientID int64) (bool, error) {
    return true, nil
}
func (f *fakeRepo) ListPrescriptions(ctx context.Context, filter ListPrescriptionsFilter) ([]Prescription, error) {
    return []Prescription{}, nil
}
func (f *fakeRepo) ListPatientsForPhysician(ctx context.Context, physicianID int64) ([]Patient, error) {
    return []Patient{}, nil
}
func (f *fakeRepo) FindOrCreateDrug(ctx context.Context, name string) (int64, error) {
    // return a dummy id for tests
    return 1, nil
}
func (f *fakeRepo) ListPhysiciansForPatient(ctx context.Context, patientID int64) ([]Physician, error) {
    return []Physician{}, nil
}

func TestHandleTopDrugs(t *testing.T) {
    now := time.Now().UTC()
    from := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)
    to := now.Add(24 * time.Hour).Format(time.RFC3339)

    cases := []struct {
        name         string
        role         string
        userID       string
        limitParam   string
        expectStatus int
        expectLimit  int
        expectScoped bool // patient scope expected
    }{
        {name: "admin default limit", role: "admin", userID: "1", limitParam: "", expectStatus: http.StatusOK, expectLimit: 10, expectScoped: false},
        {name: "custom limit", role: "admin", userID: "1", limitParam: "5", expectStatus: http.StatusOK, expectLimit: 5, expectScoped: false},
        {name: "patient scoped", role: "patient", userID: "42", limitParam: "", expectStatus: http.StatusOK, expectLimit: 10, expectScoped: true},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            fr := &fakeRepo{top: []TopDrug{{DrugID: 1, DrugName: "Ibuprofen", TotalQty: 30}}}
            srv := NewServer(fr)

            req := httptest.NewRequest(http.MethodGet, "/analytics/top-drugs?from="+from+"&to="+to+func() string { if tc.limitParam != "" { return "&limit="+tc.limitParam }; return "" }() , nil)
            req.Header.Set("X-Role", tc.role)
            if tc.userID != "" { req.Header.Set("X-User-ID", tc.userID) }

            rr := httptest.NewRecorder()
            srv.ServeHTTP(rr, req)

            if rr.Code != tc.expectStatus {
                t.Fatalf("status = %d, want %d, body=%s", rr.Code, tc.expectStatus, rr.Body.String())
            }
            if rr.Code != http.StatusOK { return }

            // Validate repo received proper parameters
            if tc.expectLimit != fr.gotLimit {
                t.Fatalf("limit passed to repo = %d, want %d", fr.gotLimit, tc.expectLimit)
            }
            if tc.expectScoped && fr.gotPatientID == nil {
                t.Fatalf("expected patient scoping, got nil patientID")
            }
            if !tc.expectScoped && fr.gotPatientID != nil {
                t.Fatalf("did not expect patient scoping, got %v", *fr.gotPatientID)
            }

            // Response JSON sanity
            var resp struct {
                Items []TopDrug `json:"items"`
            }
            if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&resp); err != nil {
                t.Fatalf("invalid json: %v", err)
            }
            if len(resp.Items) != 1 || resp.Items[0].DrugName != "Ibuprofen" {
                t.Fatalf("unexpected items: %+v", resp.Items)
            }
        })
    }
}
