import { z } from "zod";

export const imageStatusSchema = z.enum(["queued", "processing", "processed", "needs_review", "failed"]);
export type ImageStatus = z.infer<typeof imageStatusSchema>;

export const imageMetadataSchema = z.object({
  subject: z.string().min(1),
  category: z.string().min(1),
  attributes: z.array(z.string()),
  caption: z.string().min(1),
  confidence: z.number().min(0).max(1),
});
export type ImageMetadata = z.infer<typeof imageMetadataSchema>;

export const imageRecordSchema = z.object({
  id: z.string().uuid(),
  status: imageStatusSchema,
  sha256: z.string().length(64),
  mime_type: z.enum(["image/jpeg", "image/png"]),
  size_bytes: z.number().int().positive(),
  metadata: imageMetadataSchema.optional(),
  created_at: z.string().datetime({ offset: true }),
});
export type ImageRecord = z.infer<typeof imageRecordSchema>;

export const imagePageSchema = z.object({
  items: z.array(imageRecordSchema),
  total: z.number().int().nonnegative(),
  limit: z.number().int().positive(),
  offset: z.number().int().nonnegative(),
  status_counts: z.record(z.string(), z.number().int().nonnegative()).default({}),
});
export type ImagePage = z.infer<typeof imagePageSchema>;

export const imageUploadSchema = z.object({
  image_id: z.string().uuid(),
  job_id: z.string().uuid().optional(),
  status: imageStatusSchema,
  existing: z.boolean(),
});
export type ImageUploadResult = z.infer<typeof imageUploadSchema>;

export const healthSchema = z.object({ status: z.literal("ok") });
export type HealthResponse = z.infer<typeof healthSchema>;

export const readinessSchema = z.object({
  status: z.enum(["ready", "not_ready"]),
  checks: z.object({ database: z.enum(["ok", "unavailable"]) }),
});
export type ReadinessResponse = z.infer<typeof readinessSchema>;

export const reviewActionSchema = z.object({
  id: z.string().uuid(),
  decision: z.enum(["approved", "rejected"]),
  comment: z.string().optional(),
  created_at: z.string().datetime({ offset: true }),
});
export type ReviewAction = z.infer<typeof reviewActionSchema>;

export const recentMatchSchema = z.object({
  suggestion_id: z.string().uuid(),
  post_id: z.string().uuid(),
  post_title: z.string(),
  post_subject: z.string(),
  subject_confidence: z.number().min(0).max(1),
  image_id: z.string().uuid(),
  rank: z.number().int().positive(),
  subject: z.string().optional(),
  category: z.string().optional(),
  caption: z.string().optional(),
  confidence: z.number().min(0).max(1),
  uncertain: z.boolean(),
  similarity: z.number(),
  accepted: z.boolean(),
  rejection_reason: z.string().optional(),
  created_at: z.string().datetime({ offset: true }),
  latest_review: reviewActionSchema.optional(),
  review_count: z.number().int().nonnegative(),
});
export type RecentMatch = z.infer<typeof recentMatchSchema>;

export const matchPageSchema = z.object({
  items: z.array(recentMatchSchema),
  total: z.number().int().nonnegative(),
  limit: z.number().int().positive(),
  offset: z.number().int().nonnegative(),
});
export type MatchPage = z.infer<typeof matchPageSchema>;

export const reviewHistoryPageSchema = z.object({
  items: z.array(reviewActionSchema),
  total: z.number().int().nonnegative(),
  limit: z.number().int().positive(),
  offset: z.number().int().nonnegative(),
});
export type ReviewHistoryPage = z.infer<typeof reviewHistoryPageSchema>;

export const postSchema = z.object({
  id: z.string().uuid(),
  title: z.string(),
  subject: z.string(),
  subject_confidence: z.number().min(0).max(1),
  created_at: z.string().datetime({ offset: true }),
});
export type Post = z.infer<typeof postSchema>;

export const matchCandidateSchema = z.object({
  image_id: z.string().uuid(),
  subject: z.string(),
  category: z.string(),
  similarity: z.number(),
  confidence: z.number().min(0).max(1),
  uncertain: z.boolean(),
  rank: z.number().int().positive(),
  accepted: z.boolean(),
  rejection_reason: z.string().optional(),
  suggestion_id: z.string().uuid().optional(),
});
export type MatchCandidate = z.infer<typeof matchCandidateSchema>;

export const matchResultSchema = z.object({
  status: z.string(),
  suggestions: z.array(matchCandidateSchema),
  rejected: z.array(matchCandidateSchema).optional(),
  reasons: z.array(z.string()),
});
export type MatchResult = z.infer<typeof matchResultSchema>;

export const reviewResultSchema = reviewActionSchema;
export type ReviewResult = z.infer<typeof reviewResultSchema>;

export type ApiErrorEnvelope = {
  error?: { code?: string; message?: string; request_id?: string };
};
