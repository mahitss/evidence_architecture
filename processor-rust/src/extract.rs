use crate::pdf::PdfExtraction;

/// Maximum extracted UTF-8 text payload retained in PostgreSQL (5 MB ceiling per LOW_LEVEL_DESIGN.md §2).
pub const MAX_EXTRACTED_TEXT_BYTES: usize = 5 * 1024 * 1024;

/// Merged structured extraction output persisted to PostgreSQL `evidence_items`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ExtractedDocument {
    pub text: String,
    pub page_count: i32,
    pub char_count: usize,
    pub ocr_used: bool,
}

/// Merges native PDF text extraction with optional OCR page outputs.
///
/// Rules:
/// - If `ocr_pages` is `Some`, uses the OCR output page-by-page (falling back to any native text on a page if OCR was empty), and marks `ocr_used = true`.
/// - Otherwise concatenates native PDF pages separated by double newlines, and marks `ocr_used = false`.
/// - Truncates safely on a UTF-8 character boundary if total length exceeds `MAX_EXTRACTED_TEXT_BYTES`.
pub fn merge_extraction(pdf: &PdfExtraction, ocr_pages: Option<&[String]>) -> ExtractedDocument {
    let (merged, ocr_used) = match ocr_pages {
        Some(ocr) => {
            let mut combined = Vec::with_capacity(pdf.page_count.max(ocr.len()));
            for idx in 0..pdf.page_count.max(ocr.len()) {
                let ocr_text = ocr.get(idx).map(|s| s.trim()).unwrap_or("");
                let pdf_text = pdf.pages.get(idx).map(|p| p.text.trim()).unwrap_or("");
                if !ocr_text.is_empty() {
                    combined.push(ocr_text.to_string());
                } else if !pdf_text.is_empty() {
                    combined.push(pdf_text.to_string());
                }
            }
            (combined.join("\n\n"), true)
        }
        None => {
            let combined: Vec<&str> = pdf
                .pages
                .iter()
                .map(|p| p.text.trim())
                .filter(|s| !s.is_empty())
                .collect();
            (combined.join("\n\n"), false)
        }
    };

    let text = truncate_utf8_bytes(&merged, MAX_EXTRACTED_TEXT_BYTES);
    let char_count = text.chars().filter(|c| !c.is_whitespace()).count();
    let page_count = i32::try_from(pdf.page_count.max(1)).unwrap_or(i32::MAX);

    ExtractedDocument {
        text,
        page_count,
        char_count,
        ocr_used,
    }
}

/// Wraps single-page plain-text or image OCR text into an `ExtractedDocument`.
pub fn from_single_page(raw_text: &str, ocr_used: bool) -> ExtractedDocument {
    let trimmed = raw_text.trim();
    let text = truncate_utf8_bytes(trimmed, MAX_EXTRACTED_TEXT_BYTES);
    let char_count = text.chars().filter(|c| !c.is_whitespace()).count();
    ExtractedDocument {
        text,
        page_count: 1,
        char_count,
        ocr_used,
    }
}

fn truncate_utf8_bytes(s: &str, max_bytes: usize) -> String {
    if s.len() <= max_bytes {
        return s.to_string();
    }
    let mut end = max_bytes;
    while end > 0 && !s.is_char_boundary(end) {
        end -= 1;
    }
    s[..end].to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::pdf::PageText;

    #[test]
    fn merges_digital_pdf_without_ocr() {
        let pdf = PdfExtraction {
            page_count: 2,
            has_text_layer: true,
            pages: vec![
                PageText {
                    page_no: 1,
                    text: "Page one content".to_string(),
                    char_count: 14,
                },
                PageText {
                    page_no: 2,
                    text: "Page two content".to_string(),
                    char_count: 14,
                },
            ],
        };

        let merged = merge_extraction(&pdf, None);
        assert_eq!(merged.page_count, 2);
        assert!(!merged.ocr_used);
        assert_eq!(merged.text, "Page one content\n\nPage two content");
        assert_eq!(merged.char_count, 28);
    }

    #[test]
    fn merges_ocr_pages_when_scanned_and_sets_ocr_used() {
        let pdf = PdfExtraction {
            page_count: 2,
            has_text_layer: false,
            pages: vec![
                PageText {
                    page_no: 1,
                    text: String::new(),
                    char_count: 0,
                },
                PageText {
                    page_no: 2,
                    text: "fallback footer".to_string(),
                    char_count: 14,
                },
            ],
        };
        let ocr = vec!["OCR page one text".to_string(), "   ".to_string()];

        let merged = merge_extraction(&pdf, Some(&ocr));
        assert_eq!(merged.page_count, 2);
        assert!(merged.ocr_used);
        assert_eq!(merged.text, "OCR page one text\n\nfallback footer");
    }
}
