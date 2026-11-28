package search

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mahitss/evidence_architecture/backend-go/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFilterExpressions_T5SecurityGuards(t *testing.T) {
	t.Parallel()

	exprs, err := BuildFilterExpressions(SearchFilters{
		OnlyPublished: true,
		Category:      "financial_crime",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{`status = "PUBLISHED"`, `category = "financial_crime"`}, exprs)

	// Malicious filter-injection attempt via category must be rejected before Meili is contacted.
	_, err = BuildFilterExpressions(SearchFilters{
		OnlyPublished: true,
		Category:      `financial_crime" OR status = "DRAFT`,
	})
	assert.ErrorIs(t, err, ErrInvalidCategory)
}

func TestMeiliClient_IndexingSearchFiltersAndDelete(t *testing.T) {
	meiliURL := strings.TrimSpace(os.Getenv("MEILI_URL"))
	if meiliURL == "" {
		meiliURL = "http://127.0.0.1:7700"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Create isolated per-test index name so parallel or repeat runs stay pristine.
	indexName := "cases_test_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]

	now := time.Now().UTC().Truncate(time.Second)
	finCaseID := uuid.New()
	histCaseID := uuid.New()
	draftCaseID := uuid.New()

	finText := "Forensic accounting ledger traces offshore Cayman wire transfers totaling $42.5 million via shell entity Meridian Holdings."
	histText := "Declassified 1962 Treasury archival memorandum regarding sovereign bullion transport."
	draftText := "CONFIDENTIAL UNPUBLISHED DRAFT mentioning Meridian Holdings."

	casesByID := map[uuid.UUID]*models.Case{
		finCaseID: {
			ID:               finCaseID,
			Slug:             "us-v-meridian-fraud-2026",
			Title:            "United States v. Meridian Financial Holdings",
			OfficialBriefing: "Federal securities fraud prosecution involving wire transfers.",
			Category:         "financial_crime",
			Status:           "PUBLISHED",
			PublishedAt:      &now,
		},
		histCaseID: {
			ID:               histCaseID,
			Slug:             "treasury-archive-1962",
			Title:            "1962 Treasury Bullion Archival Dossier",
			OfficialBriefing: "Historical public record inquiry into bullion ledger discrepancies.",
			Category:         "historical_public_record",
			Status:           "PUBLISHED",
			PublishedAt:      &now,
		},
		draftCaseID: {
			ID:               draftCaseID,
			Slug:             "unpublished-draft-secret",
			Title:            "Unpublished Draft Meridian Investigation",
			OfficialBriefing: "Internal unreviewed draft dossier.",
			Category:         "financial_crime",
			Status:           "DRAFT",
		},
	}

	evidenceByCase := map[uuid.UUID][]*models.EvidenceItem{
		finCaseID: {
			{
				ID:            uuid.New(),
				CaseID:        finCaseID,
				Title:         "Exhibit A — Cayman Wire Transfer Ledger",
				SourceType:    "gov_primary_record",
				ExtractedText: &finText,
			},
		},
		histCaseID: {
			{
				ID:            uuid.New(),
				CaseID:        histCaseID,
				Title:         "Exhibit 1 — Treasury Memo 1962",
				SourceType:    "historical_public_domain",
				ExtractedText: &histText,
			},
		},
		draftCaseID: {
			{
				ID:            uuid.New(),
				CaseID:        draftCaseID,
				Title:         "Draft Exhibit — Unreviewed",
				SourceType:    "gov_primary_record",
				ExtractedText: &draftText,
			},
		},
	}

	src := StoreDataSource{
		GetCase: func(_ context.Context, id uuid.UUID) (*models.Case, error) {
			return casesByID[id], nil
		},
		ListEvidence: func(_ context.Context, caseID uuid.UUID) ([]*models.EvidenceItem, error) {
			return evidenceByCase[caseID], nil
		},
	}

	client, err := NewClient(ctx, meiliURL, os.Getenv("MEILI_API_KEY"), indexName, src)
	if err != nil {
		t.Fatalf("Meilisearch not reachable at %s (start via `make meili-up`): %v", meiliURL, err)
	}
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanCancel()
		_ = client.DeleteIndex(cleanCtx)
	})

	require.NoError(t, client.Health(ctx))

	// Index all 3 cases (including a DRAFT case to prove status="PUBLISHED" filter excludes non-published even if present).
	require.NoError(t, client.IndexCase(ctx, finCaseID))
	require.NoError(t, client.IndexCase(ctx, histCaseID))
	require.NoError(t, client.IndexCase(ctx, draftCaseID))

	t.Run("search finds published case by title and excludes non-published DRAFT", func(t *testing.T) {
		res, err := client.Search(ctx, "Meridian", SearchFilters{OnlyPublished: true}, 20, 0)
		require.NoError(t, err)
		require.Len(t, res.Hits, 1)
		assert.Equal(t, "us-v-meridian-fraud-2026", res.Hits[0].Slug)
		assert.Contains(t, res.Hits[0].Highlight, "<mark>")
	})

	t.Run("search finds published case by extracted_text content", func(t *testing.T) {
		res, err := client.Search(ctx, "Cayman", SearchFilters{OnlyPublished: true}, 20, 0)
		require.NoError(t, err)
		require.Len(t, res.Hits, 1)
		assert.Equal(t, finCaseID.String(), res.Hits[0].ID)
		assert.Contains(t, res.Hits[0].Highlight, "<mark>Cayman</mark>")
	})

	t.Run("filter by category narrows results cleanly", func(t *testing.T) {
		// Both financial and historical docs contain "ledger".
		allRes, err := client.Search(ctx, "ledger", SearchFilters{OnlyPublished: true}, 20, 0)
		require.NoError(t, err)
		assert.Len(t, allRes.Hits, 2)

		histOnly, err := client.Search(ctx, "ledger", SearchFilters{
			OnlyPublished: true,
			Category:      "historical_public_record",
		}, 20, 0)
		require.NoError(t, err)
		require.Len(t, histOnly.Hits, 1)
		assert.Equal(t, "treasury-archive-1962", histOnly.Hits[0].Slug)
	})

	t.Run("T5 injection string in query is treated strictly as search text, never leaking DRAFT", func(t *testing.T) {
		res, err := client.Search(ctx, `Meridian" OR status = "DRAFT`, SearchFilters{OnlyPublished: true}, 20, 0)
		require.NoError(t, err)
		for _, hit := range res.Hits {
			assert.NotEqual(t, draftCaseID.String(), hit.ID, "DRAFT case must never leak via query injection")
		}
	})

	t.Run("query exceeding 200 chars is rejected", func(t *testing.T) {
		_, err := client.Search(ctx, strings.Repeat("a", 201), SearchFilters{OnlyPublished: true}, 20, 0)
		assert.ErrorIs(t, err, ErrInvalidQuery)
	})

	t.Run("DeleteCase removes published case from search immediately", func(t *testing.T) {
		require.NoError(t, client.DeleteCase(ctx, finCaseID))
		res, err := client.Search(ctx, "Cayman", SearchFilters{OnlyPublished: true}, 20, 0)
		require.NoError(t, err)
		assert.Empty(t, res.Hits)
	})
}
