use lopdf::Document;

use crate::error::PdfError;

/// Minimum non-whitespace character threshold across all pages to classify a PDF as having a usable text layer.
pub const MIN_TEXT_LAYER_CHARS: usize = 20;

/// Hard page ceiling to defend against pathological/decompression-bomb PDFs (LOW_LEVEL_DESIGN.md §2).
pub const MAX_PDF_PAGES: usize = 500;

/// Extracted text for a single PDF page.
///
/// Follow-up note: Per-word bounding boxes (`bbox`) are intentionally omitted at MVP;
/// extracting glyph-level bounding boxes requires full layout/font-metric reconstruction
/// and is deferred to a post-MVP enhancement.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PageText {
    pub page_no: usize,
    pub text: String,
    pub char_count: usize,
}

/// Result of parsing a PDF document's embedded text layer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PdfExtraction {
    pub page_count: usize,
    pub has_text_layer: bool,
    pub pages: Vec<PageText>,
}

/// Parses raw PDF bytes using pure-Rust `lopdf` (zero native shared library dependency;
/// chosen as fallback because `libpdfium` is unavailable in the sandbox host).
///
/// Detection logic:
/// - Validates the `%PDF-` header signature and parses all pages in ascending page order.
/// - Counts non-whitespace characters across all pages.
/// - If total character count < `MIN_TEXT_LAYER_CHARS` (20), sets `has_text_layer = false`
///   so the caller invokes OCR fallback (Phase G).
pub fn extract_text(pdf_bytes: &[u8]) -> Result<PdfExtraction, PdfError> {
    if !pdf_bytes.starts_with(b"%PDF-") {
        return Err(PdfError::InvalidHeader);
    }

    let doc = Document::load_mem(pdf_bytes).map_err(|e| PdfError::Corrupt(e.to_string()))?;
    let pages_map = doc.get_pages();
    let page_count = pages_map.len();

    if page_count == 0 {
        return Err(PdfError::EmptyDocument);
    }
    if page_count > MAX_PDF_PAGES {
        return Err(PdfError::PageLimitExceeded(page_count, MAX_PDF_PAGES));
    }

    let mut pages = Vec::with_capacity(page_count);
    let mut total_chars = 0usize;

    for &page_num in pages_map.keys() {
        let raw_text = doc.extract_text(&[page_num]).unwrap_or_default();
        let cleaned = normalize_page_text(&raw_text);
        let char_count = cleaned.chars().filter(|c| !c.is_whitespace()).count();
        total_chars += char_count;

        pages.push(PageText {
            page_no: page_num as usize,
            text: cleaned,
            char_count,
        });
    }

    let has_text_layer = total_chars >= MIN_TEXT_LAYER_CHARS;

    Ok(PdfExtraction {
        page_count,
        has_text_layer,
        pages,
    })
}

fn normalize_page_text(input: &str) -> String {
    input
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty())
        .collect::<Vec<_>>()
        .join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    fn fixture_bytes(name: &str) -> Vec<u8> {
        let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("tests")
            .join("fixtures")
            .join(name);
        std::fs::read(&path).unwrap_or_else(|e| panic!("failed to read fixture {:?}: {}", path, e))
    }

    #[test]
    fn extracts_digital_text_pdf_with_text_layer() {
        let bytes = fixture_bytes("digital-text.pdf");
        let extracted = extract_text(&bytes).expect("digital PDF should parse cleanly");

        assert_eq!(extracted.page_count, 2);
        assert!(extracted.has_text_layer, "expected has_text_layer=true");
        let total_chars: usize = extracted.pages.iter().map(|p| p.char_count).sum();
        assert!(
            total_chars >= MIN_TEXT_LAYER_CHARS,
            "expected char_count >= {}, got {}",
            MIN_TEXT_LAYER_CHARS,
            total_chars
        );
        assert!(extracted.pages[0]
            .text
            .contains("UNITED STATES DISTRICT COURT"));
        assert!(extracted.pages[1].text.contains("Wire Transfer Ledger"));
    }

    #[test]
    fn detects_scanned_image_only_pdf_without_text_layer() {
        let bytes = fixture_bytes("scanned.pdf");
        let extracted = extract_text(&bytes).expect("scanned PDF should parse cleanly");

        assert_eq!(extracted.page_count, 2);
        assert!(
            !extracted.has_text_layer,
            "expected scanned image-only PDF to set has_text_layer=false"
        );
        let total_chars: usize = extracted.pages.iter().map(|p| p.char_count).sum();
        assert_eq!(total_chars, 0);
    }

    #[test]
    fn rejects_non_pdf_or_corrupt_bytes() {
        let err = extract_text(b"not a pdf at all").unwrap_err();
        assert!(matches!(err, PdfError::InvalidHeader));

        let err2 = extract_text(b"%PDF-1.4\ncorrupt garbage bytes without trailer").unwrap_err();
        assert!(matches!(err2, PdfError::Corrupt(_)));
    }
}
