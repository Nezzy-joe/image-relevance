import { z } from "zod";
import {
  healthSchema,
  imagePageSchema,
  imageRecordSchema,
  imageUploadSchema,
  matchPageSchema,
  matchResultSchema,
  postSchema,
  readinessSchema,
  reviewResultSchema,
  reviewHistoryPageSchema,
  type ImagePage,
  type ImageRecord,
  type ImageStatus,
  type ImageUploadResult,
  type MatchPage,
  type MatchResult,
  type Post,
  type ReadinessResponse,
  type ReviewResult,
  type ReviewHistoryPage,
} from "../types/api";

const API_ROOT = "/api";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;

  constructor(message: string, status: number, code = "request_failed", requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

async function requestJSON<T>(path: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${API_ROOT}${path}`, {
      ...init,
      headers: { Accept: "application/json", ...init?.headers },
    });
  } catch {
    throw new ApiError("The local API could not be reached. Check that the Go server is running.", 0, "offline");
  }

  const requestId = response.headers.get("X-Request-ID") ?? undefined;
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    payload = undefined;
  }

  if (!response.ok) {
    const envelope = payload as { error?: { code?: string; message?: string; request_id?: string } } | undefined;
    throw new ApiError(
      envelope?.error?.message ?? `The request failed with HTTP ${response.status}.`,
      response.status,
      envelope?.error?.code ?? "request_failed",
      envelope?.error?.request_id ?? requestId,
    );
  }

  const parsed = schema.safeParse(payload);
  if (!parsed.success) {
    throw new ApiError("The API returned a response that did not match its documented format.", response.status, "invalid_response", requestId);
  }
  return parsed.data;
}

export const api = {
  health: () => requestJSON("/health", healthSchema),
  readiness: () => requestJSON<ReadinessResponse>("/ready", readinessSchema),
  listImages: (params: {
    limit?: number;
    offset?: number;
    q?: string;
    status?: ImageStatus | "";
    category?: string;
    subject?: string;
  } = {}): Promise<ImagePage> => {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== "") query.set(key, String(value));
    }
    const serialized = query.toString();
    const suffix = serialized ? `?${serialized}` : "";
    return requestJSON(`/v1/images${suffix}`, imagePageSchema);
  },
  getImage: (imageID: string): Promise<ImageRecord> => requestJSON(`/v1/images/${encodeURIComponent(imageID)}`, imageRecordSchema),
  uploadImage: (file: File): Promise<ImageUploadResult> => {
    const form = new FormData();
    form.append("file", file);
    return requestJSON("/v1/images", imageUploadSchema, { method: "POST", body: form });
  },
  listMatches: (limit = 20, offset = 0): Promise<MatchPage> =>
    requestJSON(`/v1/matches?limit=${limit}&offset=${offset}`, matchPageSchema),
  listReviewHistory: (suggestionID: string, limit = 20, offset = 0): Promise<ReviewHistoryPage> =>
    requestJSON(`/v1/matches/${encodeURIComponent(suggestionID)}/reviews?limit=${limit}&offset=${offset}`, reviewHistoryPageSchema),
  createPost: (input: { title: string; body: string }): Promise<Post> =>
    requestJSON("/v1/posts", postSchema, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) }),
  matchPost: (postID: string): Promise<MatchResult> =>
    requestJSON(`/v1/posts/${encodeURIComponent(postID)}/matches`, matchResultSchema, { method: "POST" }),
  submitReview: (suggestionID: string, input: { decision: "approved" | "rejected"; comment?: string }): Promise<ReviewResult> =>
    requestJSON(`/v1/matches/${encodeURIComponent(suggestionID)}/review`, reviewResultSchema, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) }),
  imageContentURL: (imageID: string) => `${API_ROOT}/v1/images/${encodeURIComponent(imageID)}/content`,
};
