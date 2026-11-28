use thiserror::Error;

/// Classification of job errors determining retry vs. immediate DLQ routing.
#[derive(Debug, Error)]
pub enum ProcessorError {
    /// Transient failure (Redis/Postgres/IO blip) — leave message unacknowledged for retry up to max attempts.
    #[error("transient error ({code}): {message}")]
    Transient { code: &'static str, message: String },

    /// Permanent failure (corrupt PDF, unsupported MIME, OCR unavailable, path violation) — route immediately to DLQ.
    #[error("permanent error ({code}): {message}")]
    Permanent { code: &'static str, message: String },
}

impl ProcessorError {
    pub fn transient(code: &'static str, message: impl Into<String>) -> Self {
        Self::Transient {
            code,
            message: message.into(),
        }
    }

    pub fn permanent(code: &'static str, message: impl Into<String>) -> Self {
        Self::Permanent {
            code,
            message: message.into(),
        }
    }

    pub fn is_permanent(&self) -> bool {
        matches!(self, Self::Permanent { .. })
    }

    pub fn code(&self) -> &'static str {
        match self {
            Self::Transient { code, .. } | Self::Permanent { code, .. } => code,
        }
    }
}

/// PDF parsing failure taxonomy.
#[derive(Debug, Error)]
pub enum PdfError {
    #[error("invalid or missing PDF header")]
    InvalidHeader,
    #[error("corrupt or unreadable PDF: {0}")]
    Corrupt(String),
    #[error("PDF exceeds maximum page count limit ({0} > {1})")]
    PageLimitExceeded(usize, usize),
    #[error("PDF contains zero pages")]
    EmptyDocument,
}

/// OCR subprocess failure taxonomy.
#[derive(Debug, Error)]
pub enum OcrError {
    #[error("OCR unavailable")]
    Unavailable,
    #[error("invalid OCR language specification: {0}")]
    InvalidLanguage(String),
    #[error("OCR rasterization failed: {0}")]
    RasterizationFailed(String),
    #[error("tesseract subprocess timed out after {0}s")]
    Timeout(u64),
    #[error("tesseract subprocess failed: {0}")]
    ProcessFailed(String),
}
