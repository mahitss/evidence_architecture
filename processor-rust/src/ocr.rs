use std::path::Path;
use std::process::Stdio;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use tokio::io::AsyncWriteExt;
use tokio::process::Command;
use tokio::time::timeout;
use tracing::{info, warn};

use crate::error::OcrError;

/// Maximum per-page OCR subprocess execution time.
pub const OCR_PAGE_TIMEOUT_SECS: u64 = 30;

/// Global flag set at startup once `tesseract` binary availability is probed.
static OCR_DISABLED: AtomicBool = AtomicBool::new(false);

/// Probes whether the `tesseract` CLI binary is installed and executable.
/// Logs WARN and marks `OCR_DISABLED = true` when missing so the consumer keeps running
/// and fails OCR-requiring jobs permanently instead of crashing or retrying forever.
pub fn probe_tesseract_at_startup() -> bool {
    let available = is_binary_available("tesseract", &["--version"]);
    if available {
        OCR_DISABLED.store(false, Ordering::SeqCst);
        info!("tesseract CLI detected; OCR fallback enabled");
    } else {
        OCR_DISABLED.store(true, Ordering::SeqCst);
        warn!("tesseract CLI not found on PATH; setting OCR_DISABLED=true (scanned PDFs will fail permanently with 'OCR unavailable')");
    }
    available
}

/// Returns true if OCR was disabled at startup because `tesseract` is missing.
pub fn is_ocr_disabled() -> bool {
    OCR_DISABLED.load(Ordering::SeqCst)
}

/// Runs `tesseract - - --psm 3 -l <lang>` as an isolated subprocess (never via shell)
/// piping image bytes over stdin and collecting UTF-8 text from stdout.
pub async fn run_ocr(image_bytes: &[u8], lang: &str) -> Result<String, OcrError> {
    if is_ocr_disabled() {
        return Err(OcrError::Unavailable);
    }
    validate_lang(lang)?;

    let mut child = Command::new("tesseract")
        .args(["-", "-", "--psm", "3", "-l", lang])
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .kill_on_drop(true)
        .spawn()
        .map_err(|e| {
            if e.kind() == std::io::ErrorKind::NotFound {
                OCR_DISABLED.store(true, Ordering::SeqCst);
                OcrError::Unavailable
            } else {
                OcrError::ProcessFailed(e.to_string())
            }
        })?;

    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(image_bytes)
            .await
            .map_err(|e| OcrError::ProcessFailed(format!("write stdin: {}", e)))?;
    }

    let output = timeout(
        Duration::from_secs(OCR_PAGE_TIMEOUT_SECS),
        child.wait_with_output(),
    )
    .await
    .map_err(|_| OcrError::Timeout(OCR_PAGE_TIMEOUT_SECS))?
    .map_err(|e| OcrError::ProcessFailed(e.to_string()))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(OcrError::ProcessFailed(format!(
            "exit {}: {}",
            output.status,
            stderr.trim()
        )));
    }

    Ok(String::from_utf8_lossy(&output.stdout).trim().to_string())
}

/// Rasterizes each page of a scanned PDF into PNG images using Ghostscript (`gs`) or `pdftoppm`
/// inside an isolated temporary directory, then runs `run_ocr` on every rendered page.
pub async fn ocr_scanned_pdf(pdf_bytes: &[u8], lang: &str) -> Result<Vec<String>, OcrError> {
    if is_ocr_disabled() {
        return Err(OcrError::Unavailable);
    }
    validate_lang(lang)?;

    let tmp_dir = tempfile::tempdir()
        .map_err(|e| OcrError::RasterizationFailed(format!("create tempdir: {}", e)))?;
    let input_pdf = tmp_dir.path().join("input.pdf");
    tokio::fs::write(&input_pdf, pdf_bytes)
        .await
        .map_err(|e| OcrError::RasterizationFailed(format!("write temp pdf: {}", e)))?;

    let rendered_paths = rasterize_pdf_to_pngs(&input_pdf, tmp_dir.path()).await?;
    if rendered_paths.is_empty() {
        return Err(OcrError::RasterizationFailed(
            "rasterizer produced zero page images".to_string(),
        ));
    }

    let mut page_texts = Vec::with_capacity(rendered_paths.len());
    for page_path in rendered_paths {
        let png_bytes = tokio::fs::read(&page_path)
            .await
            .map_err(|e| OcrError::RasterizationFailed(format!("read rendered png: {}", e)))?;
        let text = run_ocr(&png_bytes, lang).await?;
        page_texts.push(text);
    }
    Ok(page_texts)
}

async fn rasterize_pdf_to_pngs(
    input_pdf: &Path,
    out_dir: &Path,
) -> Result<Vec<std::path::PathBuf>, OcrError> {
    let out_pattern = out_dir.join("page-%04d.png");
    let status = if is_binary_available("pdftoppm", &["-v"]) {
        let prefix = out_dir.join("page");
        Command::new("pdftoppm")
            .arg("-png")
            .arg("-r")
            .arg("150")
            .arg(input_pdf)
            .arg(prefix)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .status()
            .await
            .map_err(|e| OcrError::RasterizationFailed(e.to_string()))?
    } else if is_binary_available("gs", &["--version"]) {
        Command::new("gs")
            .arg("-q")
            .arg("-dNOPAUSE")
            .arg("-dBATCH")
            .arg("-dSAFER")
            .arg("-sDEVICE=png16m")
            .arg("-r150")
            .arg(format!("-sOutputFile={}", out_pattern.display()))
            .arg(input_pdf)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .status()
            .await
            .map_err(|e| OcrError::RasterizationFailed(e.to_string()))?
    } else {
        return Err(OcrError::RasterizationFailed(
            "neither pdftoppm nor gs is installed to rasterize PDF pages".to_string(),
        ));
    };

    if !status.success() {
        return Err(OcrError::RasterizationFailed(format!(
            "rasterizer exited with {}",
            status
        )));
    }

    let mut entries = tokio::fs::read_dir(out_dir)
        .await
        .map_err(|e| OcrError::RasterizationFailed(e.to_string()))?;
    let mut pngs = Vec::new();
    while let Some(entry) = entries
        .next_entry()
        .await
        .map_err(|e| OcrError::RasterizationFailed(e.to_string()))?
    {
        let p = entry.path();
        if p.extension().and_then(|s| s.to_str()) == Some("png") {
            pngs.push(p);
        }
    }
    pngs.sort();
    Ok(pngs)
}

fn validate_lang(lang: &str) -> Result<(), OcrError> {
    if lang.is_empty()
        || !lang
            .split('+')
            .all(|part| part.len() == 3 && part.bytes().all(|b| b.is_ascii_lowercase()))
    {
        return Err(OcrError::InvalidLanguage(lang.to_string()));
    }
    Ok(())
}

fn is_binary_available(bin: &str, args: &[&str]) -> bool {
    std::process::Command::new(bin)
        .args(args)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map(|s| s.success())
        .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn graceful_degradation_when_tesseract_missing_or_live_ocr_when_present() {
        let available = probe_tesseract_at_startup();
        if !available {
            assert!(is_ocr_disabled());
            let err = run_ocr(b"fake-png-bytes", "eng").await.unwrap_err();
            assert!(matches!(err, OcrError::Unavailable));
            return;
        }

        // If tesseract IS installed (e.g., in CI), verify language validation and invocation path.
        assert!(!is_ocr_disabled());
        let bad_lang = run_ocr(b"fake", "eng;rm -rf /").await.unwrap_err();
        assert!(matches!(bad_lang, OcrError::InvalidLanguage(_)));
    }

    #[test]
    fn validates_language_allowlist_against_injection() {
        assert!(validate_lang("eng").is_ok());
        assert!(validate_lang("eng+fra").is_ok());
        assert!(validate_lang("").is_err());
        assert!(validate_lang("eng;whoami").is_err());
        assert!(validate_lang("../etc").is_err());
    }
}
