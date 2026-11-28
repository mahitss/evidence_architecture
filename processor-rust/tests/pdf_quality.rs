//! PDF Extraction Quality Benchmark (Phase G / ADR-018)
//!
//! NOTE: These fixtures are synthetic/representative PDFs, not real government PDFs.
//! Full corpus validation against real DOJ/SEC/PACER PDFs runs in CI (ADR-018).
//!
//! Each fixture MUST yield:
//!   - `extracted_text.len() > 500` characters
//!   - `distinct_words > 100` unique normalized tokens
//!
//! Do NOT weaken these thresholds. If `lopdf` cannot satisfy them on real-world PDFs in CI
//! (< 90% recall across the CI corpus), supersede ADR-018 and migrate to `pdfium-render`.

use std::collections::HashSet;
use std::path::PathBuf;

use processor_rust::pdf::extract_text;

const MIN_EXTRACTED_CHARS: usize = 500;
const MIN_DISTINCT_WORDS: usize = 100;

fn count_distinct_words(text: &str) -> usize {
    text.split(|c: char| !c.is_alphanumeric())
        .map(|w| w.trim().to_ascii_lowercase())
        .filter(|w| w.len() >= 2)
        .collect::<HashSet<_>>()
        .len()
}

fn fixture_path(name: &str) -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join(name)
}

#[test]
fn pdf_extraction_quality_benchmark_meets_minimum_thresholds() {
    let fixtures = [
        ("gov-press-release.pdf", "meridian"),
        ("court-docket-page.pdf", "harborline"),
        ("historical-record.pdf", "congressional"),
    ];

    for (filename, expected_keyword) in fixtures {
        let path = fixture_path(filename);
        let bytes = std::fs::read(&path)
            .unwrap_or_else(|err| panic!("failed to read fixture {}: {}", path.display(), err));

        let extracted = extract_text(&bytes)
            .unwrap_or_else(|err| panic!("lopdf extraction failed for {}: {}", filename, err));

        let combined_text = extracted
            .pages
            .iter()
            .map(|p| p.text.as_str())
            .collect::<Vec<_>>()
            .join("\n\n");

        let char_count = combined_text.len();
        let distinct_words = count_distinct_words(&combined_text);

        println!(
            "[pdf_quality] fixture={} pages={} has_text_layer={} extracted_chars={} distinct_words={}",
            filename, extracted.page_count, extracted.has_text_layer, char_count, distinct_words
        );

        assert!(
            extracted.has_text_layer,
            "digital fixture {} should have a usable text layer without OCR fallback",
            filename
        );
        assert!(
            extracted.page_count >= 2,
            "fixture {} expected >= 2 pages, got {}",
            filename,
            extracted.page_count
        );
        assert!(
            char_count > MIN_EXTRACTED_CHARS,
            "fixture {} extracted {} chars (must be > {})",
            filename,
            char_count,
            MIN_EXTRACTED_CHARS
        );
        assert!(
            distinct_words > MIN_DISTINCT_WORDS,
            "fixture {} extracted {} distinct words (must be > {})",
            filename,
            distinct_words,
            MIN_DISTINCT_WORDS
        );
        assert!(
            combined_text
                .to_ascii_lowercase()
                .contains(expected_keyword),
            "fixture {} missing expected domain keyword {:?}",
            filename,
            expected_keyword
        );
    }
}
