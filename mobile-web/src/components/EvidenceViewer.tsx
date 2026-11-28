import { useEffect, useRef, useState } from 'react';
import { Button, Callout, Card } from './ui';
import { formatBytes, isSafeHttpUrl } from '../lib/format';
import type { EvidenceDetail } from '../lib/schemas';

/**
 * PDF Viewer Library Evaluation (Phase A):
 * - Evaluated `react-pdf` vs `pdfjs-dist` directly.
 * - Selected `pdfjs-dist` because:
 *   1. `react-pdf` adds a wrapper layer with stricter React peer-dependency constraints (React 19 friction)
 *      and duplicates PDF.js worker wiring.
 *   2. Direct dynamic `import('pdfjs-dist')` isolates PDF parsing into an on-demand async chunk so
 *      the initial mobile PWA bundle stays well below the 500 KB gzipped target.
 *   3. When canvas/worker rendering is unavailable (offline signed URL expiry, restrictive CSP, or jsdom),
 *      the component degrades gracefully to a direct download / external view link.
 */

export interface EvidenceViewerProps {
  evidence: EvidenceDetail;
}

export function EvidenceViewer({ evidence }: EvidenceViewerProps) {
  const rawFileUrl = evidence.signed_file_url ?? evidence.storage_url;
  const fileUrl = isSafeHttpUrl(rawFileUrl) ? rawFileUrl : null;
  const mime = (evidence.mime_type || '').toLowerCase();
  const isPdf = mime === 'application/pdf' || Boolean(fileUrl && /\.pdf(\?|$)/i.test(fileUrl));
  const isImage = mime.startsWith('image/');

  if (evidence.is_redacted) {
    return (
      <Callout variant="warning" title="Exhibit Withheld / Redacted">
        <p className="m-0">
          This file is withheld from public display.
          {evidence.redaction_reason ? ` Reason: ${evidence.redaction_reason}` : ''}
        </p>
      </Callout>
    );
  }

  if (!fileUrl) {
    return (
      <Card tone="light" className="p-5">
        <p className="m-0 text-sm text-[var(--ink-secondary)]">
          No downloadable file attachment is currently linked to this evidence record. Refer to the
          source citation above for the primary record.
        </p>
      </Card>
    );
  }

  if (isPdf) {
    return (
      <PdfDocumentViewer
        url={fileUrl}
        title={evidence.title}
        byteSize={evidence.byte_size}
        pageCount={evidence.page_count}
      />
    );
  }

  if (isImage) {
    return <ImageEvidenceViewer url={fileUrl} title={evidence.title} byteSize={evidence.byte_size} />;
  }

  return (
    <Card tone="light" className="p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div className="space-y-1">
        <p className="m-0 text-sm font-semibold text-[var(--ink)]">{evidence.title}</p>
        <p className="m-0 text-xs font-[var(--font-mono)] text-[var(--ink-secondary)]">
          {evidence.mime_type || 'application/octet-stream'} · {formatBytes(evidence.byte_size)}
        </p>
      </div>
      <a
        href={fileUrl}
        download
        rel="noopener noreferrer"
        className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--accent)] text-[var(--ink-on-dark)] rounded-[var(--radius-md)] hover:bg-[var(--accent-hover)] no-underline"
      >
        Download file
      </a>
    </Card>
  );
}

function ImageEvidenceViewer({
  url,
  title,
  byteSize,
}: {
  url: string;
  title: string;
  byteSize: number;
}) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <Card tone="light" className="p-5 space-y-3">
        <p className="m-0 text-sm text-[var(--ink-secondary)]">
          Unable to preview this image inline.
        </p>
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center justify-center min-h-[44px] px-4 py-2 text-sm font-medium bg-[var(--accent)] text-[var(--ink-on-dark)] rounded-[var(--radius-md)] no-underline"
        >
          Open image directly ({formatBytes(byteSize)})
        </a>
      </Card>
    );
  }

  return (
    <Card tone="light" className="p-4 space-y-3">
      <figure className="m-0 space-y-2">
        <img
          src={url}
          alt={title}
          onError={() => setFailed(true)}
          className="w-full max-h-[70vh] object-contain bg-[var(--surface-dark)] rounded-[var(--radius-sm)] border border-[var(--surface-border-subtle)]"
        />
        <figcaption className="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--ink-secondary)]">
          <span>{title}</span>
          <a
            href={url}
            download
            rel="noopener noreferrer"
            className="underline text-[var(--accent-text)] hover:text-[var(--ink)]"
          >
            Download original ({formatBytes(byteSize)})
          </a>
        </figcaption>
      </figure>
    </Card>
  );
}

function PdfDocumentViewer({
  url,
  title,
  byteSize,
  pageCount,
}: {
  url: string;
  title: string;
  byteSize: number;
  pageCount: number | null;
}) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [totalPages, setTotalPages] = useState<number>(pageCount ?? 1);
  const [status, setStatus] = useState<'loading' | 'ready' | 'fallback'>('loading');

  useEffect(() => {
    let cancelled = false;
    let renderTask: { cancel: () => void } | null = null;

    async function renderPdfPage() {
      // jsdom does not implement HTMLCanvasElement.getContext('2d'); fall back cleanly
      const canvas = canvasRef.current;
      if (!canvas || typeof canvas.getContext !== 'function') {
        setStatus('fallback');
        return;
      }

      const ctx = canvas.getContext('2d');
      if (!ctx) {
        setStatus('fallback');
        return;
      }

      try {
        setStatus('loading');
        const pdfjs = await import('pdfjs-dist');
        if (!pdfjs.GlobalWorkerOptions.workerSrc) {
          pdfjs.GlobalWorkerOptions.workerSrc = new URL(
            'pdfjs-dist/build/pdf.worker.min.mjs',
            import.meta.url,
          ).toString();
        }

        const loadingTask = pdfjs.getDocument({ url });
        const pdfDoc = await loadingTask.promise;
        if (cancelled) {
          await pdfDoc.destroy();
          return;
        }

        setTotalPages(pdfDoc.numPages);
        const clampedPage = Math.min(Math.max(1, pageNumber), pdfDoc.numPages);
        const page = await pdfDoc.getPage(clampedPage);
        if (cancelled) {
          await pdfDoc.destroy();
          return;
        }

        const viewport = page.getViewport({ scale: 1.35 });
        canvas.height = viewport.height;
        canvas.width = viewport.width;

        const task = page.render({ canvasContext: ctx, viewport });
        renderTask = task;
        await task.promise;
        if (!cancelled) {
          setStatus('ready');
        }
        await pdfDoc.destroy();
      } catch {
        if (!cancelled) {
          setStatus('fallback');
        }
      }
    }

    void renderPdfPage();

    return () => {
      cancelled = true;
      renderTask?.cancel();
    };
  }, [url, pageNumber]);

  return (
    <Card tone="light" className="p-4 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3 pb-3 border-b border-[var(--surface-border-subtle)]">
        <div className="space-y-0.5">
          <p className="m-0 text-sm font-semibold text-[var(--ink)]">PDF Document Viewer</p>
          <p className="m-0 text-xs font-[var(--font-mono)] text-[var(--ink-secondary)]">
            {totalPages} {totalPages === 1 ? 'page' : 'pages'} · {formatBytes(byteSize)}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {status === 'ready' && totalPages > 1 && (
            <div className="flex items-center gap-1.5">
              <Button
                variant="secondary"
                size="sm"
                disabled={pageNumber <= 1}
                onClick={() => setPageNumber((p) => Math.max(1, p - 1))}
                aria-label="Previous PDF page"
              >
                Prev
              </Button>
              <span className="text-xs font-[var(--font-mono)] px-1.5 text-[var(--ink)]">
                {pageNumber} / {totalPages}
              </span>
              <Button
                variant="secondary"
                size="sm"
                disabled={pageNumber >= totalPages}
                onClick={() => setPageNumber((p) => Math.min(totalPages, p + 1))}
                aria-label="Next PDF page"
              >
                Next
              </Button>
            </div>
          )}
          <a
            href={url}
            download
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center min-h-[38px] px-3 py-1.5 text-sm font-medium bg-[var(--accent)] text-[var(--ink-on-dark)] rounded-[var(--radius-md)] hover:bg-[var(--accent-hover)] no-underline"
          >
            Download PDF
          </a>
        </div>
      </div>

      {status === 'fallback' ? (
        <div className="p-5 bg-[var(--paper)] border border-[var(--surface-border-subtle)] rounded-[var(--radius-sm)] space-y-2 text-sm">
          <p className="m-0 font-medium text-[var(--ink)]">
            Inline PDF preview is unavailable in this browser session.
          </p>
          <p className="m-0 text-[var(--ink-secondary)]">
            Use the button above to open or download <span className="font-medium">{title}</span>{' '}
            directly.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto bg-[#e5e2d9] border border-[var(--surface-border-subtle)] rounded-[var(--radius-sm)] p-2 flex justify-center">
          <canvas
            ref={canvasRef}
            aria-label={`PDF preview of ${title}, page ${pageNumber} of ${totalPages}`}
            className={`max-w-full h-auto shadow-sm ${status === 'loading' ? 'opacity-50' : 'opacity-100'}`}
          />
        </div>
      )}
    </Card>
  );
}
