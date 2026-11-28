package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/mahitss/evidence_architecture/backend-go/internal/search"
	"github.com/mahitss/evidence_architecture/backend-go/internal/store"
)

type stubReaderCaseStore struct {
	summaries []store.PublishedCaseSummary
	bySlug    map[string]*models.Case
	timeline  []models.TimelineEvent
}

func (s *stubReaderCaseStore) ListPublished(_ context.Context, _ string, _ string, _ int) ([]store.PublishedCaseSummary, *string, bool, error) {
	return s.summaries, nil, false, nil
}

func (s *stubReaderCaseStore) GetPublishedBySlug(_ context.Context, slug string) (*models.Case, error) {
	if c, ok := s.bySlug[slug]; ok {
		return c, nil
	}
	return nil, store.ErrNotFound
}

func (s *stubReaderCaseStore) ListTimelineByCaseID(_ context.Context, _ uuid.UUID) ([]models.TimelineEvent, error) {
	return s.timeline, nil
}

type stubReaderEvidenceStore struct {
	itemsByCase map[uuid.UUID][]*models.EvidenceItem
	byID        map[uuid.UUID]*models.EvidenceItem
}

func (s *stubReaderEvidenceStore) ListByCaseID(_ context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error) {
	return s.itemsByCase[caseID], nil
}

func (s *stubReaderEvidenceStore) GetPublishedByID(_ context.Context, id uuid.UUID) (*models.EvidenceItem, error) {
	if ev, ok := s.byID[id]; ok {
		return ev, nil
	}
	return nil, store.ErrNotFound
}

type stubCorrectionStore struct {
	createdCalls int
	lastInput    store.CreateCorrectionInput
}

func (s *stubCorrectionStore) Create(_ context.Context, in store.CreateCorrectionInput) (*models.CorrectionRequest, error) {
	s.createdCalls++
	s.lastInput = in
	return &models.CorrectionRequest{
		ID:                    uuid.New(),
		TicketID:              "COR-TEST0001",
		Status:                "OPEN",
		RequestType:           in.RequestType,
		RequesterContactEmail: in.RequesterContactEmail,
		Description:           in.Description,
		CreatedAt:             time.Now().UTC(),
	}, nil
}

type stubReaderStorage struct{}

func (stubReaderStorage) Put(_ context.Context, key string, _ io.Reader, _ string) (string, error) {
	return "file:///local/" + key, nil
}
func (stubReaderStorage) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (stubReaderStorage) Delete(_ context.Context, _ string) error { return nil }
func (stubReaderStorage) URL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "file:///local/" + key + "?expires=300", nil
}

type stubSearchIndexer struct {
	lastQuery   string
	lastFilters search.SearchFilters
	result      *search.SearchResult
	err         error
}

func (s *stubSearchIndexer) IndexCase(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubSearchIndexer) IndexCaseDocument(_ context.Context, _ *models.Case, _ []*models.EvidenceItem) error {
	return nil
}
func (s *stubSearchIndexer) DeleteCase(_ context.Context, _ uuid.UUID) error { return nil }
func (s *stubSearchIndexer) Health(_ context.Context) error                  { return nil }
func (s *stubSearchIndexer) Search(_ context.Context, query string, filters search.SearchFilters, _, _ int) (*search.SearchResult, error) {
	s.lastQuery = query
	s.lastFilters = filters
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func TestReaderListCasesRejectsDraftStatusParam(t *testing.T) {
	h := NewReaderHandler(&stubReaderCaseStore{}, &stubReaderEvidenceStore{}, &stubCorrectionStore{}, stubReaderStorage{}, &stubSearchIndexer{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cases?status=DRAFT", nil)
	rec := httptest.NewRecorder()
	h.ListCases(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when status=DRAFT is requested publicly, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestReaderGetCaseBySlugReturns404ForUnknownOrUnpublished(t *testing.T) {
	h := NewReaderHandler(&stubReaderCaseStore{bySlug: map[string]*models.Case{}}, &stubReaderEvidenceStore{}, &stubCorrectionStore{}, stubReaderStorage{}, &stubSearchIndexer{})

	r := chi.NewRouter()
	r.Get("/api/v1/cases/{slug}", h.GetCaseBySlug)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cases/unpublished-draft-case", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestReaderGetEvidenceRedactedSuppressesSignedURL(t *testing.T) {
	evID := uuid.New()
	reason := "Court-ordered redaction"
	evStore := &stubReaderEvidenceStore{
		byID: map[uuid.UUID]*models.EvidenceItem{
			evID: {
				ID:               evID,
				CaseID:           uuid.New(),
				Title:            "Redacted Filing",
				SourceType:       "gov_primary_record",
				VerificationTier: 1,
				LicenseStatus:    "PUBLIC_DOMAIN",
				ObjectKey:        "cases/x/evidence/y/hash.pdf",
				IsRedacted:       true,
				RedactionReason:  &reason,
				RetrievedAt:      time.Now().UTC(),
			},
		},
	}

	h := NewReaderHandler(&stubReaderCaseStore{}, evStore, &stubCorrectionStore{}, stubReaderStorage{}, &stubSearchIndexer{})
	r := chi.NewRouter()
	r.Get("/api/v1/evidence/{id}", h.GetEvidenceByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evidence/"+evID.String(), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["signed_file_url"] != nil || body["storage_url"] != nil {
		t.Fatalf("redacted evidence MUST NOT expose signed URL: %+v", body)
	}
}

func TestReaderSearchEnforcesBoundsAndPublishedFilter(t *testing.T) {
	idx := &stubSearchIndexer{
		result: &search.SearchResult{
			Hits: []search.SearchHit{{ID: uuid.NewString(), Slug: "sec-case", Title: "SEC Case", Highlight: "<mark>wire</mark> fraud"}},
		},
	}
	h := NewReaderHandler(&stubReaderCaseStore{}, &stubReaderEvidenceStore{}, &stubCorrectionStore{}, stubReaderStorage{}, idx)

	// Missing q -> 400
	rec := httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/api/v1/search", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty q, got %d", rec.Code)
	}

	// Over 200 chars -> 400
	longQ := strings.Repeat("a", 201)
	rec = httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+longQ, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 201-char q, got %d", rec.Code)
	}

	// Injection category -> 400
	rec = httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=fraud&category=financial_crime+OR+status%3DDRAFT", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for injected category, got %d", rec.Code)
	}

	// Valid query -> 200 and OnlyPublished=true
	rec = httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/api/v1/search?q=wire+fraud&category=financial_crime", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid search, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !idx.lastFilters.OnlyPublished || idx.lastFilters.Category != "financial_crime" {
		t.Fatalf("expected OnlyPublished=true and Category=financial_crime, got %+v", idx.lastFilters)
	}
}

func TestReaderSubmitCorrectionHoneypotSilentDrop(t *testing.T) {
	corrStore := &stubCorrectionStore{}
	h := NewReaderHandler(&stubReaderCaseStore{}, &stubReaderEvidenceStore{}, corrStore, stubReaderStorage{}, &stubSearchIndexer{})

	// Bot submission with honeypot_field filled -> silent 202 and zero DB writes.
	botBody := `{"case_slug":"test-case","description":"Buy cheap watches right now online!","honeypot_field":"http://spam.example"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corrections", bytes.NewBufferString(botBody))
	rec := httptest.NewRecorder()
	h.SubmitCorrection(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected silent 202 for honeypot submission, got %d (%s)", rec.Code, rec.Body.String())
	}
	if corrStore.createdCalls != 0 {
		t.Fatalf("expected 0 DB writes when honeypot is filled, got %d", corrStore.createdCalls)
	}

	// Legitimate submission -> 202 and persisted.
	validBody := `{"case_slug":"test-case","description":"The filing date on Exhibit 2 should read March 14, 2022.","contact_email":"reader@example.com"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/corrections", bytes.NewBufferString(validBody))
	rec2 := httptest.NewRecorder()
	h.SubmitCorrection(rec2, req2)

	if rec2.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for valid correction, got %d (%s)", rec2.Code, rec2.Body.String())
	}
	if corrStore.createdCalls != 1 {
		t.Fatalf("expected 1 DB write for valid correction, got %d", corrStore.createdCalls)
	}
	if corrStore.lastInput.RequesterContactEmail != "reader@example.com" {
		t.Fatalf("unexpected email: %q", corrStore.lastInput.RequesterContactEmail)
	}
}
