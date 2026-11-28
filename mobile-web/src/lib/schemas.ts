import { z } from 'zod';

export const CaseCategorySchema = z.string().min(1);

export const CaseSummarySchema = z.object({
  id: z.string().min(1),
  slug: z.string().min(1),
  title: z.string().min(1),
  summary: z.string().default(''),
  category: CaseCategorySchema,
  jurisdiction: z.string().optional().default(''),
  published_at: z.string().nullable().optional().default(null),
  updated_at: z.string().optional(),
  evidence_count: z.number().int().nonnegative().default(0),
});

export type CaseSummary = z.infer<typeof CaseSummarySchema>;

export const CaseListResponseSchema = z.object({
  data: z.array(CaseSummarySchema),
  next_cursor: z.string().nullable().optional().default(null),
  has_more: z.boolean().optional().default(false),
});

export type CaseListResponse = z.infer<typeof CaseListResponseSchema>;

export const GenuineGapObjectSchema = z
  .object({
    claim: z.string().optional(),
    why_unverified: z.string().optional(),
    question: z.string().optional(),
    description: z.string().optional(),
  })
  .passthrough();

export const GenuineGapEntrySchema = z.union([z.string(), GenuineGapObjectSchema]);

export type GenuineGapEntry = z.infer<typeof GenuineGapEntrySchema>;

export const EvidenceSummarySchema = z.object({
  id: z.string().min(1),
  title: z.string().min(1),
  description: z.string().optional().default(''),
  type: z.string().optional().default('document'),
  source_type: z.string().optional().default('gov_primary_record'),
  verification_tier: z.number().int().min(1).max(4),
  license_status: z.string().min(1),
  source_citation: z.string().optional().default(''),
  sha256_hash: z.string().optional().default(''),
  is_redacted: z.boolean().optional().default(false),
  warning_level: z.string().optional().default('NONE'),
});

export type EvidenceSummary = z.infer<typeof EvidenceSummarySchema>;

export const TimelineEventSchema = z.object({
  id: z.string().min(1),
  case_id: z.string().optional(),
  event_start_date: z.string().min(1),
  event_end_date: z.string().nullable().optional().default(null),
  sort_order: z.number().int().optional().default(0),
  event_type: z.string().optional().default('event'),
  title: z.string().min(1),
  summary: z.string().optional().default(''),
  verification_tier: z.number().int().min(1).max(4).optional().default(1),
  has_conflict: z.boolean().optional().default(false),
  controlling_status_note: z.string().nullable().optional().default(null),
});

export type TimelineEvent = z.infer<typeof TimelineEventSchema>;

export const CaseDetailSchema = z.object({
  id: z.string().min(1),
  slug: z.string().min(1),
  title: z.string().min(1),
  summary: z.string().optional().default(''),
  official_briefing: z.string().optional().default(''),
  category: CaseCategorySchema,
  jurisdiction: z.string().optional().default(''),
  published_at: z.string().nullable().optional().default(null),
  updated_at: z.string().optional(),
  evidence_count: z.number().int().nonnegative().optional().default(0),
  genuine_gaps: z.array(GenuineGapEntrySchema).optional().default([]),
  genuine_gaps_note: z.string().optional().default(''),
  evidence_items: z.array(EvidenceSummarySchema).optional().default([]),
  timeline: z.array(TimelineEventSchema).optional().default([]),
  timeline_events: z.array(TimelineEventSchema).optional().default([]),
});

export type CaseDetail = z.infer<typeof CaseDetailSchema>;

export const EvidenceDetailSchema = z.object({
  id: z.string().min(1),
  case_id: z.string().optional().default(''),
  title: z.string().min(1),
  description: z.string().optional().default(''),
  type: z.string().optional().default('document'),
  source_type: z.string().optional().default('gov_primary_record'),
  verification_tier: z.number().int().min(1).max(4),
  license_status: z.string().min(1),
  license_basis_note: z.string().optional().default(''),
  source_citation: z.string().optional().default(''),
  issuing_authority: z.string().optional().default(''),
  source_url: z.string().nullable().optional().default(null),
  storage_url: z.string().nullable().optional().default(null),
  signed_file_url: z.string().nullable().optional().default(null),
  signed_url_expires_at: z.string().nullable().optional().default(null),
  recreation_author_note: z.string().nullable().optional().default(null),
  mime_type: z.string().optional().default('application/octet-stream'),
  byte_size: z.number().int().nonnegative().optional().default(0),
  page_count: z.number().int().nullable().optional().default(null),
  ocr_used: z.boolean().optional().default(false),
  sha256_hash: z.string().optional().default(''),
  is_redacted: z.boolean().optional().default(false),
  redaction_reason: z.string().nullable().optional().default(null),
  warning_level: z.string().optional().default('NONE'),
  retrieved_at: z.string().nullable().optional().default(null),
});

export type EvidenceDetail = z.infer<typeof EvidenceDetailSchema>;

export const SearchHitSchema = z.object({
  id: z.string().min(1),
  slug: z.string().min(1),
  title: z.string().min(1),
  summary: z.string().optional().default(''),
  category: CaseCategorySchema,
  published_at: z.string().nullable().optional().default(null),
  evidence_count: z.number().int().nonnegative().optional().default(0),
  highlight: z.string().optional().default(''),
});

export type SearchHit = z.infer<typeof SearchHitSchema>;

export const SearchResponseSchema = z.object({
  data: z.array(SearchHitSchema),
  next_cursor: z.string().nullable().optional().default(null),
  has_more: z.boolean().optional().default(false),
});

export type SearchResponse = z.infer<typeof SearchResponseSchema>;

export const SubmitCorrectionFormSchema = z.object({
  case_slug: z.string().trim().min(1, 'Please select or enter a case.'),
  description: z
    .string()
    .trim()
    .min(20, 'Please provide at least 20 characters describing the factual issue or source.')
    .max(2000, 'Description must be 2,000 characters or fewer.'),
  contact_email: z
    .string()
    .trim()
    .refine(
      (val) => val === '' || z.string().email().safeParse(val).success,
      'Please enter a valid email address or leave this field blank.',
    ),
  honeypot_field: z.string().optional().default(''),
});

export type SubmitCorrectionInput = z.infer<typeof SubmitCorrectionFormSchema>;

export const SubmitCorrectionResponseSchema = z.object({
  correction_id: z.string().min(1),
  ticket_id: z.string().optional().default(''),
  status: z.string().optional().default('OPEN'),
  created_at: z.string().optional(),
});

export type SubmitCorrectionResponse = z.infer<typeof SubmitCorrectionResponseSchema>;

export const ErrorEnvelopeSchema = z.object({
  error: z
    .object({
      code: z.string().optional(),
      message: z.string().optional(),
      request_id: z.string().optional(),
    })
    .optional(),
  message: z.string().optional(),
  code: z.string().optional(),
});
