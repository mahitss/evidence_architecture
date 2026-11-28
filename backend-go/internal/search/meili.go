package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	meilisearch "github.com/meilisearch/meilisearch-go"
)

const (
	// DefaultIndexName is the single MVP Meilisearch index per ADR-019.
	DefaultIndexName = "cases"
	// MaxQueryLength enforces SECURITY_ARCHITECTURE.md T5 query bound.
	MaxQueryLength = 200
)

var (
	// ErrInvalidQuery is returned when a search query is empty or exceeds MaxQueryLength.
	ErrInvalidQuery = errors.New("search query must be between 1 and 200 characters")
	// ErrInvalidCategory is returned when a category filter is outside the strict launch enum allowlist.
	ErrInvalidCategory = errors.New("category must be financial_crime or historical_public_record")
)

var allowedCategories = map[string]struct{}{
	"financial_crime":          {},
	"historical_public_record": {},
}

// CaseDataSource fetches case and evidence records needed to build an index document.
type CaseDataSource interface {
	GetCaseByID(ctx context.Context, id uuid.UUID) (*models.Case, error)
	ListEvidenceByCaseID(ctx context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error)
}

// StoreDataSource adapts store.CaseStore and store.EvidenceStore into CaseDataSource.
type StoreDataSource struct {
	GetCase      func(ctx context.Context, id uuid.UUID) (*models.Case, error)
	ListEvidence func(ctx context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error)
}

// GetCaseByID delegates to GetCase.
func (s StoreDataSource) GetCaseByID(ctx context.Context, id uuid.UUID) (*models.Case, error) {
	return s.GetCase(ctx, id)
}

// ListEvidenceByCaseID delegates to ListEvidence.
func (s StoreDataSource) ListEvidenceByCaseID(ctx context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error) {
	return s.ListEvidence(ctx, caseID)
}

// CaseDocument is the Meilisearch document schema defined in ADR-019.
type CaseDocument struct {
	ID                    string   `json:"id"`
	CaseSlug              string   `json:"case_slug"`
	CaseTitle             string   `json:"case_title"`
	CaseSummary           string   `json:"case_summary"`
	Status                string   `json:"status"`
	Category              string   `json:"category"`
	PublishedAt           string   `json:"published_at"`
	EvidenceCount         int      `json:"evidence_count"`
	EvidenceTitles        []string `json:"evidence_titles"`
	EvidenceExtractedText []string `json:"evidence_extracted_text"`
	EvidenceSourceType    []string `json:"evidence_source_type"`
}

// SearchFilters restricts search results using strictly allowlisted server-side filter expressions (T5).
type SearchFilters struct {
	Category      string
	OnlyPublished bool
}

// SearchHit represents a single matched published case with highlighted snippet.
type SearchHit struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Category      string `json:"category"`
	PublishedAt   string `json:"published_at"`
	EvidenceCount int    `json:"evidence_count"`
	Highlight     string `json:"highlight"`
}

// SearchResult holds paginated hits from Meilisearch.
type SearchResult struct {
	Hits            []SearchHit
	EstimatedTotal  int64
	HasMore         bool
	ProcessingTimeM int64
}

// Indexer defines search indexing and querying operations consumed by API handlers.
type Indexer interface {
	IndexCase(ctx context.Context, caseID uuid.UUID) error
	IndexCaseDocument(ctx context.Context, c *models.Case, items []*models.EvidenceItem) error
	DeleteCase(ctx context.Context, caseID uuid.UUID) error
	Search(ctx context.Context, query string, filters SearchFilters, limit, offset int) (*SearchResult, error)
	Health(ctx context.Context) error
}

// Client wraps the meilisearch-go SDK with ADR-019 settings and T5 query safety.
type Client struct {
	svc       meilisearch.ServiceManager
	indexName string
	source    CaseDataSource
}

// NewClient initializes a Meilisearch client targeting baseURL and ensures index settings per ADR-019.
func NewClient(ctx context.Context, baseURL, apiKey, indexName string, source CaseDataSource) (*Client, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("meilisearch base URL is required")
	}
	if strings.TrimSpace(indexName) == "" {
		indexName = DefaultIndexName
	}

	var opts []meilisearch.Option
	if apiKey != "" {
		opts = append(opts, meilisearch.WithAPIKey(apiKey))
	}
	svc := meilisearch.New(baseURL, opts...)

	c := &Client{
		svc:       svc,
		indexName: indexName,
		source:    source,
	}

	if err := c.EnsureIndex(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// EnsureIndex creates the index if absent and applies ADR-019 filterable, searchable, and ranking settings.
func (c *Client) EnsureIndex(ctx context.Context) error {
	task, err := c.svc.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{
		Uid:        c.indexName,
		PrimaryKey: "id",
	})
	if err == nil && task != nil {
		// Wait for index creation (if it already exists, Meilisearch completes with index_already_exists).
		_, _ = c.svc.WaitForTaskWithContext(ctx, task.TaskUID, 25*time.Millisecond)
	}

	settings := &meilisearch.Settings{
		FilterableAttributes: []string{"status", "category"},
		SearchableAttributes: []string{
			"case_title",
			"case_summary",
			"evidence_titles",
			"evidence_extracted_text",
		},
		RankingRules: []string{
			"exactness",
			"words",
			"typo",
			"proximity",
			"attribute",
			"sort",
		},
	}

	updTask, err := c.svc.Index(c.indexName).UpdateSettingsWithContext(ctx, settings)
	if err != nil {
		return fmt.Errorf("update meilisearch settings for index %q: %w", c.indexName, err)
	}
	if err := c.waitSucceeded(ctx, updTask.TaskUID, "update settings"); err != nil {
		return err
	}
	return nil
}

// Health pings Meilisearch and confirms status == "available".
func (c *Client) Health(ctx context.Context) error {
	h, err := c.svc.HealthWithContext(ctx)
	if err != nil {
		return fmt.Errorf("meilisearch health check: %w", err)
	}
	if h == nil || h.Status != "available" {
		return fmt.Errorf("meilisearch unhealthy: %+v", h)
	}
	return nil
}

// IndexCase fetches a case and its evidence items from the database and upserts the document into Meilisearch.
func (c *Client) IndexCase(ctx context.Context, caseID uuid.UUID) error {
	if c.source == nil {
		return errors.New("case data source is not configured")
	}
	caseRec, err := c.source.GetCaseByID(ctx, caseID)
	if err != nil {
		return fmt.Errorf("fetch case %s for indexing: %w", caseID, err)
	}
	items, err := c.source.ListEvidenceByCaseID(ctx, caseID)
	if err != nil {
		return fmt.Errorf("fetch evidence for case %s indexing: %w", caseID, err)
	}
	return c.IndexCaseDocument(ctx, caseRec, items)
}

// IndexCaseDocument builds an ADR-019 CaseDocument from already-loaded models and synchronously upserts it.
func (c *Client) IndexCaseDocument(ctx context.Context, caseRec *models.Case, items []*models.EvidenceItem) error {
	if caseRec == nil {
		return errors.New("case record is nil")
	}
	doc := BuildCaseDocument(caseRec, items)

	task, err := c.svc.Index(c.indexName).AddDocumentsWithContext(ctx, []CaseDocument{doc}, "id")
	if err != nil {
		return fmt.Errorf("meilisearch add document %s: %w", caseRec.ID, err)
	}
	return c.waitSucceeded(ctx, task.TaskUID, "index case "+caseRec.ID.String())
}

// DeleteCase synchronously removes caseID from the Meilisearch index.
func (c *Client) DeleteCase(ctx context.Context, caseID uuid.UUID) error {
	task, err := c.svc.Index(c.indexName).DeleteDocumentWithContext(ctx, caseID.String())
	if err != nil {
		return fmt.Errorf("meilisearch delete document %s: %w", caseID, err)
	}
	return c.waitSucceeded(ctx, task.TaskUID, "delete case "+caseID.String())
}

// Search executes a T5-hardened Meilisearch query.
// User input `query` is passed strictly as Meilisearch's structured `q` search term and NEVER
// interpolated into filter syntax. All filter values are strictly allowlisted server-side.
func (c *Client) Search(ctx context.Context, query string, filters SearchFilters, limit, offset int) (*SearchResult, error) {
	trimmed := strings.TrimSpace(query)
	if len(trimmed) == 0 || len(trimmed) > MaxQueryLength {
		return nil, ErrInvalidQuery
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	filterExprs, err := BuildFilterExpressions(filters)
	if err != nil {
		return nil, err
	}

	req := &meilisearch.SearchRequest{
		Limit:                 int64(limit + 1),
		Offset:                int64(offset),
		AttributesToHighlight: []string{"case_title", "case_summary", "evidence_titles", "evidence_extracted_text"},
		AttributesToCrop:      []string{"case_summary", "evidence_extracted_text"},
		CropLength:            40,
		HighlightPreTag:       "<mark>",
		HighlightPostTag:      "</mark>",
	}
	if len(filterExprs) > 0 {
		req.Filter = filterExprs
	}

	resp, err := c.svc.Index(c.indexName).SearchWithContext(ctx, trimmed, req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search: %w", err)
	}

	hasMore := false
	rawHits := resp.Hits
	if len(rawHits) > limit {
		hasMore = true
		rawHits = rawHits[:limit]
	}

	hits := make([]SearchHit, 0, len(rawHits))
	for _, raw := range rawHits {
		hit, err := decodeSearchHit(raw)
		if err != nil {
			return nil, fmt.Errorf("decode search hit: %w", err)
		}
		hits = append(hits, hit)
	}

	return &SearchResult{
		Hits:            hits,
		EstimatedTotal:  resp.EstimatedTotalHits,
		HasMore:         hasMore,
		ProcessingTimeM: resp.ProcessingTimeMs,
	}, nil
}

// DeleteIndex drops the configured index (intended for isolated test cleanup).
func (c *Client) DeleteIndex(ctx context.Context) error {
	task, err := c.svc.DeleteIndexWithContext(ctx, c.indexName)
	if err != nil {
		return err
	}
	_, _ = c.svc.WaitForTaskWithContext(ctx, task.TaskUID, 25*time.Millisecond)
	return nil
}

// BuildCaseDocument constructs the single ADR-019 Meilisearch document for a case and its evidence items.
// Redacted evidence items are excluded from searchable titles/text so redacted material is never indexed.
func BuildCaseDocument(c *models.Case, items []*models.EvidenceItem) CaseDocument {
	titles := make([]string, 0, len(items))
	texts := make([]string, 0, len(items))
	sourceTypesSeen := make(map[string]struct{})
	sourceTypes := make([]string, 0, len(items))

	for _, ev := range items {
		if ev == nil || ev.IsRedacted {
			continue
		}
		if strings.TrimSpace(ev.Title) != "" {
			titles = append(titles, ev.Title)
		}
		if ev.ExtractedText != nil && strings.TrimSpace(*ev.ExtractedText) != "" {
			texts = append(texts, *ev.ExtractedText)
		}
		if ev.SourceType != "" {
			if _, ok := sourceTypesSeen[ev.SourceType]; !ok {
				sourceTypesSeen[ev.SourceType] = struct{}{}
				sourceTypes = append(sourceTypes, ev.SourceType)
			}
		}
	}

	pubAt := ""
	if c.PublishedAt != nil {
		pubAt = c.PublishedAt.UTC().Format(time.RFC3339)
	}

	return CaseDocument{
		ID:                    c.ID.String(),
		CaseSlug:              c.Slug,
		CaseTitle:             c.Title,
		CaseSummary:           c.OfficialBriefing,
		Status:                c.Status,
		Category:              c.Category,
		PublishedAt:           pubAt,
		EvidenceCount:         len(items),
		EvidenceTitles:        titles,
		EvidenceExtractedText: texts,
		EvidenceSourceType:    sourceTypes,
	}
}

// BuildFilterExpressions constructs typed Meilisearch filter arrays using strict allowlists (T5).
func BuildFilterExpressions(filters SearchFilters) ([]string, error) {
	exprs := make([]string, 0, 2)
	if filters.OnlyPublished {
		exprs = append(exprs, `status = "PUBLISHED"`)
	}
	cat := strings.TrimSpace(filters.Category)
	if cat != "" {
		if _, ok := allowedCategories[cat]; !ok {
			return nil, ErrInvalidCategory
		}
		exprs = append(exprs, fmt.Sprintf(`category = "%s"`, cat))
	}
	return exprs, nil
}

func (c *Client) waitSucceeded(ctx context.Context, taskUID int64, op string) error {
	task, err := c.svc.WaitForTaskWithContext(ctx, taskUID, 25*time.Millisecond)
	if err != nil {
		return fmt.Errorf("wait for meilisearch task %d (%s): %w", taskUID, op, err)
	}
	if task.Status != meilisearch.TaskStatusSucceeded {
		return fmt.Errorf("meilisearch task %d (%s) status=%s error=%s", taskUID, op, task.Status, task.Error.Message)
	}
	return nil
}

func decodeSearchHit(raw any) (SearchHit, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return SearchHit{}, err
	}
	var payload struct {
		ID            string `json:"id"`
		CaseSlug      string `json:"case_slug"`
		CaseTitle     string `json:"case_title"`
		CaseSummary   string `json:"case_summary"`
		Category      string `json:"category"`
		PublishedAt   string `json:"published_at"`
		EvidenceCount int    `json:"evidence_count"`
		Formatted     struct {
			CaseTitle             string   `json:"case_title"`
			CaseSummary           string   `json:"case_summary"`
			EvidenceTitles        []string `json:"evidence_titles"`
			EvidenceExtractedText []string `json:"evidence_extracted_text"`
		} `json:"_formatted"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return SearchHit{}, err
	}

	highlight := pickBestHighlight(
		payload.Formatted.EvidenceExtractedText,
		payload.Formatted.EvidenceTitles,
		payload.Formatted.CaseSummary,
		payload.Formatted.CaseTitle,
		payload.CaseSummary,
	)

	return SearchHit{
		ID:            payload.ID,
		Slug:          payload.CaseSlug,
		Title:         payload.CaseTitle,
		Summary:       payload.CaseSummary,
		Category:      payload.Category,
		PublishedAt:   payload.PublishedAt,
		EvidenceCount: payload.EvidenceCount,
		Highlight:     highlight,
	}, nil
}

func pickBestHighlight(extractedTexts, evidenceTitles []string, summaryFmt, titleFmt, fallback string) string {
	for _, s := range extractedTexts {
		if strings.Contains(s, "<mark>") {
			return s
		}
	}
	for _, s := range evidenceTitles {
		if strings.Contains(s, "<mark>") {
			return s
		}
	}
	if strings.Contains(summaryFmt, "<mark>") {
		return summaryFmt
	}
	if strings.Contains(titleFmt, "<mark>") {
		return titleFmt
	}
	if summaryFmt != "" {
		return summaryFmt
	}
	return fallback
}
