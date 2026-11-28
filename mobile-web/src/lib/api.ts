import { v4 as uuidv4 } from 'uuid';
import { type ZodType, type ZodTypeDef } from 'zod';
import {
  CaseDetailSchema,
  CaseListResponseSchema,
  ErrorEnvelopeSchema,
  EvidenceDetailSchema,
  SearchResponseSchema,
  SubmitCorrectionResponseSchema,
  type CaseDetail,
  type CaseListResponse,
  type EvidenceDetail,
  type SearchResponse,
  type SubmitCorrectionInput,
  type SubmitCorrectionResponse,
} from './schemas';

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId: string | null;

  constructor(message: string, status: number, code = 'API_ERROR', requestId: string | null = null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

export class GeoblockError extends ApiError {
  readonly region: string | null;

  constructor(region: string | null = null, requestId: string | null = null) {
    super('Not available in your region', 451, 'UNAVAILABLE_FOR_LEGAL_REASONS', requestId);
    this.name = 'GeoblockError';
    this.region = region;
  }
}

export class RateLimitError extends ApiError {
  readonly retryAfter: number;

  constructor(retryAfter = 60, requestId: string | null = null, message?: string) {
    super(
      message ?? `Too many requests. Please try again in ${retryAfter} seconds.`,
      429,
      'RATE_LIMITED',
      requestId,
    );
    this.name = 'RateLimitError';
    this.retryAfter = retryAfter;
  }
}

export function getApiBaseUrl(): string {
  const configured = import.meta.env.VITE_API_URL;
  const base = configured && configured.trim().length > 0 ? configured.trim() : 'http://127.0.0.1:8080';
  return base.replace(/\/+$/, '');
}

function parseRetryAfter(headerValue: string | null): number {
  if (!headerValue) return 60;
  const numeric = Number.parseInt(headerValue, 10);
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric;
  }
  const parsedDate = Date.parse(headerValue);
  if (!Number.isNaN(parsedDate)) {
    const delta = Math.ceil((parsedDate - Date.now()) / 1000);
    return delta > 0 ? delta : 60;
  }
  return 60;
}

/**
 * Unwraps `{ data: T }` envelopes when present while preserving direct flat objects
 * returned by backend-go single-resource handlers.
 */
function unwrapDataEnvelope(raw: unknown): unknown {
  if (
    typeof raw === 'object' &&
    raw !== null &&
    'data' in raw &&
    typeof (raw as { data: unknown }).data === 'object' &&
    (raw as { data: unknown }).data !== null &&
    !Array.isArray((raw as { data: unknown }).data)
  ) {
    return (raw as { data: unknown }).data;
  }
  return raw;
}

async function parseErrorBody(response: Response): Promise<{
  code: string;
  message: string;
  requestId: string | null;
}> {
  const headerReqId = response.headers.get('X-Request-Id');
  const contentType = response.headers.get('content-type') ?? '';
  if (contentType.includes('application/json')) {
    try {
      const parsedJson: unknown = await response.json();
      const parsed = ErrorEnvelopeSchema.safeParse(parsedJson);
      if (parsed.success) {
        const inner = parsed.data.error;
        return {
          code: inner?.code ?? parsed.data.code ?? `HTTP_${response.status}`,
          message:
            inner?.message ??
            parsed.data.message ??
            `Request failed with status ${response.status}`,
          requestId: inner?.request_id ?? headerReqId,
        };
      }
    } catch {
      // Fall through to status text
    }
  }
  return {
    code: `HTTP_${response.status}`,
    message: response.statusText || `Request failed with status ${response.status}`,
    requestId: headerReqId,
  };
}

async function requestJson<T>(
  path: string,
  schema: ZodType<T, ZodTypeDef, unknown>,
  options?: {
    method?: 'GET' | 'POST';
    body?: unknown;
    headers?: Record<string, string>;
    signal?: AbortSignal;
    unwrapSingle?: boolean;
  },
): Promise<T> {
  const url = `${getApiBaseUrl()}${path}`;
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...options?.headers,
  };

  if (options?.body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }

  let response: Response;
  try {
    response = await fetch(url, {
      method: options?.method ?? 'GET',
      headers,
      body: options?.body !== undefined ? JSON.stringify(options.body) : undefined,
      signal: options?.signal,
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw err;
    }
    throw new ApiError(
      'Unable to reach the archive server. Check your connection and try again.',
      0,
      'NETWORK_ERROR',
      null,
    );
  }

  const requestId = response.headers.get('X-Request-Id');

  if (response.status === 451) {
    const region =
      response.headers.get('X-Geoblock-Region') ?? response.headers.get('CF-IPCountry') ?? null;
    throw new GeoblockError(region, requestId);
  }

  if (response.status === 429) {
    const retryAfter = parseRetryAfter(response.headers.get('Retry-After'));
    const errInfo = await parseErrorBody(response);
    throw new RateLimitError(retryAfter, errInfo.requestId ?? requestId, errInfo.message);
  }

  if (!response.ok) {
    const errInfo = await parseErrorBody(response);
    throw new ApiError(errInfo.message, response.status, errInfo.code, errInfo.requestId);
  }

  let rawJson: unknown;
  try {
    rawJson = await response.json();
  } catch {
    throw new ApiError('Server returned an unreadable response.', response.status, 'INVALID_JSON', requestId);
  }

  const candidate = options?.unwrapSingle ? unwrapDataEnvelope(rawJson) : rawJson;
  const validated = schema.safeParse(candidate);
  if (!validated.success) {
    throw new ApiError(
      'Received an unexpected response format from the archive API.',
      response.status,
      'SCHEMA_VALIDATION_ERROR',
      requestId,
    );
  }

  return validated.data;
}

export async function fetchCases(params: {
  category?: string;
  cursor?: string;
  limit?: number;
  signal?: AbortSignal;
}): Promise<CaseListResponse> {
  const sp = new URLSearchParams();
  if (params.category && params.category.trim().length > 0) {
    sp.set('category', params.category.trim());
  }
  if (params.cursor && params.cursor.trim().length > 0) {
    sp.set('cursor', params.cursor.trim());
  }
  if (params.limit) {
    sp.set('limit', String(params.limit));
  }
  const qs = sp.toString();
  return requestJson(`/api/v1/cases${qs ? `?${qs}` : ''}`, CaseListResponseSchema, {
    signal: params.signal,
  });
}

export async function fetchCaseBySlug(
  slug: string,
  signal?: AbortSignal,
): Promise<CaseDetail | null> {
  try {
    return await requestJson(
      `/api/v1/cases/${encodeURIComponent(slug)}`,
      CaseDetailSchema,
      {
        signal,
        unwrapSingle: true,
      },
    );
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null;
    }
    throw err;
  }
}

export async function fetchEvidenceById(
  id: string,
  signal?: AbortSignal,
): Promise<EvidenceDetail> {
  return requestJson(`/api/v1/evidence/${encodeURIComponent(id)}`, EvidenceDetailSchema, {
    signal,
    unwrapSingle: true,
  });
}

export async function searchArchive(params: {
  query: string;
  category?: string;
  cursor?: string;
  limit?: number;
  signal?: AbortSignal;
}): Promise<SearchResponse> {
  const sp = new URLSearchParams();
  sp.set('q', params.query.trim());
  if (params.category && params.category.trim().length > 0) {
    sp.set('category', params.category.trim());
  }
  if (params.cursor && params.cursor.trim().length > 0) {
    sp.set('cursor', params.cursor.trim());
  }
  if (params.limit) {
    sp.set('limit', String(params.limit));
  }
  return requestJson(`/api/v1/search?${sp.toString()}`, SearchResponseSchema, {
    signal: params.signal,
  });
}

export async function submitCorrection(
  input: SubmitCorrectionInput,
  idempotencyKey?: string,
): Promise<SubmitCorrectionResponse> {
  const key = idempotencyKey ?? uuidv4();
  return requestJson('/api/v1/corrections', SubmitCorrectionResponseSchema, {
    method: 'POST',
    headers: {
      'Idempotency-Key': key,
    },
    body: {
      case_slug: input.case_slug,
      description: input.description,
      contact_email: input.contact_email,
      honeypot_field: input.honeypot_field ?? '',
    },
    unwrapSingle: true,
  });
}
