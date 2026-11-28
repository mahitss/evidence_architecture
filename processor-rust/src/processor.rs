use std::path::{Component, Path, PathBuf};

use sha2::{Digest, Sha256};

use crate::error::{OcrError, PdfError, ProcessorError};
use crate::extract::{self, ExtractedDocument};
use crate::ocr;
use crate::pdf;

/// Hard file size ceiling matching PostgreSQL `chk_byte_size` (25 MB).
pub const MAX_EVIDENCE_FILE_BYTES: u64 = 26_214_400;

/// Orchestrates reading a local evidence object (strictly confined under `storage_root`)
/// and running MIME-appropriate text extraction / OCR fallback.
#[derive(Debug, Clone)]
pub struct DocumentProcessor {
    storage_root: PathBuf,
    ocr_available: bool,
}

impl DocumentProcessor {
    pub fn new(storage_root: PathBuf, ocr_available: bool) -> Self {
        Self {
            storage_root,
            ocr_available,
        }
    }

    pub fn storage_root(&self) -> &Path {
        &self.storage_root
    }

    /// Reads file bytes from `storage_root/storage_key` with strict path confinement and size cap,
    /// then extracts structured text via PDF parser / OCR fallback.
    pub async fn process_file(
        &self,
        storage_key: &str,
        mime_type: &str,
    ) -> Result<(ExtractedDocument, String), ProcessorError> {
        let safe_path = resolve_confined_path(&self.storage_root, storage_key)?;

        let meta = tokio::fs::metadata(&safe_path).await.map_err(|e| {
            if e.kind() == std::io::ErrorKind::NotFound {
                ProcessorError::permanent(
                    "FILE_NOT_FOUND",
                    format!("evidence file not found at storage_key {:?}", storage_key),
                )
            } else {
                ProcessorError::transient("STORAGE_IO_ERROR", format!("stat evidence file: {}", e))
            }
        })?;

        if meta.len() == 0 {
            return Err(ProcessorError::permanent(
                "EMPTY_FILE",
                "evidence file is empty",
            ));
        }
        if meta.len() > MAX_EVIDENCE_FILE_BYTES {
            return Err(ProcessorError::permanent(
                "FILE_TOO_LARGE",
                format!("evidence file size {} exceeds 25 MB limit", meta.len()),
            ));
        }

        let bytes = tokio::fs::read(&safe_path).await.map_err(|e| {
            ProcessorError::transient("STORAGE_IO_ERROR", format!("read evidence file: {}", e))
        })?;

        let sha256_hex = format!("{:x}", Sha256::digest(&bytes));
        let doc = process_bytes(&bytes, mime_type, self.ocr_available).await?;
        Ok((doc, sha256_hex))
    }
}

/// Pure extraction orchestrator operating on in-memory file bytes.
pub async fn process_bytes(
    bytes: &[u8],
    mime_type: &str,
    ocr_available: bool,
) -> Result<ExtractedDocument, ProcessorError> {
    let normalized_mime = mime_type
        .split(';')
        .next()
        .unwrap_or("")
        .trim()
        .to_ascii_lowercase();

    match normalized_mime.as_str() {
        "application/pdf" => {
            let owned = bytes.to_vec();
            let pdf_result = tokio::task::spawn_blocking(move || pdf::extract_text(&owned))
                .await
                .map_err(|e| {
                    ProcessorError::transient("WORKER_JOIN_ERROR", format!("pdf task join: {}", e))
                })?
                .map_err(map_pdf_error)?;

            if pdf_result.has_text_layer {
                return Ok(extract::merge_extraction(&pdf_result, None));
            }

            // Scanned PDF (no usable text layer) -> OCR fallback
            if !ocr_available || ocr::is_ocr_disabled() {
                return Err(ProcessorError::permanent(
                    "OCR_UNAVAILABLE",
                    "OCR unavailable: scanned PDF requires tesseract CLI",
                ));
            }

            let ocr_pages = ocr::ocr_scanned_pdf(bytes, "eng")
                .await
                .map_err(map_ocr_error)?;
            Ok(extract::merge_extraction(&pdf_result, Some(&ocr_pages)))
        }
        "image/png" | "image/jpeg" => {
            if !ocr_available || ocr::is_ocr_disabled() {
                return Err(ProcessorError::permanent(
                    "OCR_UNAVAILABLE",
                    "OCR unavailable: image evidence requires tesseract CLI",
                ));
            }
            let text = ocr::run_ocr(bytes, "eng").await.map_err(map_ocr_error)?;
            Ok(extract::from_single_page(&text, true))
        }
        "text/plain" => {
            let text = std::str::from_utf8(bytes).map_err(|e| {
                ProcessorError::permanent(
                    "INVALID_UTF8",
                    format!("text/plain evidence is not valid UTF-8: {}", e),
                )
            })?;
            Ok(extract::from_single_page(text, false))
        }
        other => Err(ProcessorError::permanent(
            "UNSUPPORTED_MIME",
            format!("unsupported mime_type {:?}", other),
        )),
    }
}

/// Resolves `storage_key` strictly underneath `storage_root`, blocking traversal (`..`),
/// absolute paths, null bytes, and symlink escapes outside `storage_root`.
pub fn resolve_confined_path(
    storage_root: &Path,
    storage_key: &str,
) -> Result<PathBuf, ProcessorError> {
    let key = storage_key.trim();
    if key.is_empty() || key.contains('\0') || key.starts_with('/') || key.starts_with('\\') {
        return Err(ProcessorError::permanent(
            "INVALID_STORAGE_KEY",
            format!("invalid storage_key {:?}", storage_key),
        ));
    }

    let rel = Path::new(key);
    for comp in rel.components() {
        match comp {
            Component::Normal(_) => {}
            _ => {
                return Err(ProcessorError::permanent(
                    "INVALID_STORAGE_KEY",
                    format!("path traversal rejected in storage_key {:?}", storage_key),
                ));
            }
        }
    }

    let root_canon = std::fs::canonicalize(storage_root).map_err(|e| {
        ProcessorError::transient(
            "STORAGE_ROOT_ERROR",
            format!("canonicalize storage_root {:?}: {}", storage_root, e),
        )
    })?;

    let joined = root_canon.join(rel);
    if joined.exists() {
        let target_canon = std::fs::canonicalize(&joined).map_err(|e| {
            ProcessorError::transient("STORAGE_IO_ERROR", format!("canonicalize target: {}", e))
        })?;
        if !target_canon.starts_with(&root_canon) {
            return Err(ProcessorError::permanent(
                "INVALID_STORAGE_KEY",
                "resolved storage path escapes storage_root",
            ));
        }
        return Ok(target_canon);
    }

    Ok(joined)
}

fn map_pdf_error(err: PdfError) -> ProcessorError {
    match err {
        PdfError::InvalidHeader => {
            ProcessorError::permanent("CORRUPT_PDF", "invalid or missing PDF header")
        }
        PdfError::Corrupt(msg) => ProcessorError::permanent("CORRUPT_PDF", msg),
        PdfError::PageLimitExceeded(n, max) => ProcessorError::permanent(
            "DECOMPRESSION_BOMB",
            format!("PDF page count {} exceeds maximum {}", n, max),
        ),
        PdfError::EmptyDocument => {
            ProcessorError::permanent("CORRUPT_PDF", "PDF contains zero pages")
        }
    }
}

fn map_ocr_error(err: OcrError) -> ProcessorError {
    match err {
        OcrError::Unavailable => ProcessorError::permanent("OCR_UNAVAILABLE", "OCR unavailable"),
        OcrError::InvalidLanguage(lang) => {
            ProcessorError::permanent("OCR_INVALID_LANGUAGE", format!("invalid lang {}", lang))
        }
        OcrError::Timeout(secs) => ProcessorError::transient(
            "OCR_TIMEOUT",
            format!("tesseract timed out after {}s", secs),
        ),
        OcrError::RasterizationFailed(msg) => {
            ProcessorError::permanent("OCR_RASTERIZATION_FAILED", msg)
        }
        OcrError::ProcessFailed(msg) => ProcessorError::transient("OCR_PROCESS_ERROR", msg),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_storage_key_path_traversal() {
        let tmp = tempfile::tempdir().unwrap();
        assert!(resolve_confined_path(tmp.path(), "../etc/passwd").is_err());
        assert!(resolve_confined_path(tmp.path(), "/etc/passwd").is_err());
        assert!(resolve_confined_path(tmp.path(), "a/../../secret.pdf").is_err());
        assert!(resolve_confined_path(tmp.path(), "").is_err());
    }

    #[tokio::test]
    async fn scanned_pdf_without_tesseract_fails_permanently_with_ocr_unavailable() {
        let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("tests")
            .join("fixtures")
            .join("scanned.pdf");
        let bytes = std::fs::read(&path).unwrap();

        let err = process_bytes(&bytes, "application/pdf", false)
            .await
            .unwrap_err();
        assert!(err.is_permanent());
        assert_eq!(err.code(), "OCR_UNAVAILABLE");
    }

    #[tokio::test]
    async fn unsupported_mime_fails_permanently() {
        let err = process_bytes(b"hello", "application/zip", false)
            .await
            .unwrap_err();
        assert!(err.is_permanent());
        assert_eq!(err.code(), "UNSUPPORTED_MIME");
    }
}
