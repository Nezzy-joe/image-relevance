import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { queryClient } from "./lib/query-client";

const postID = "33333333-3333-4333-8333-333333333333";
const foxSuggestionID = "44444444-4444-4444-8444-444444444444";
const wolfSuggestionID = "55555555-5555-4555-8555-555555555555";
const imageID = "11111111-1111-4111-8111-111111111111";
const secondImageID = "99999999-9999-4999-8999-999999999999";
const createdAt = "2026-11-13T12:00:00Z";

const emptyImagePage = {
  items: [], total: 0, limit: 24, offset: 0,
  status_counts: { queued: 0, processing: 0, processed: 0, needs_review: 0, failed: 0 },
};

const post = { id: postID, title: "Red fox field notes", subject: "red fox", subject_confidence: 0.95, created_at: createdAt };
const fox = { image_id: imageID, subject: "red fox", category: "wildlife", similarity: 0.91, confidence: 0.94, uncertain: false, rank: 1, accepted: true, suggestion_id: foxSuggestionID };
const wolf = { image_id: "22222222-2222-4222-8222-222222222222", subject: "gray wolf", category: "wildlife", similarity: 0.87, confidence: 0.96, uncertain: false, rank: 1, accepted: false, rejection_reason: "Subject mismatch: article subject 'red fox' does not match image subject 'gray wolf'.", suggestion_id: wolfSuggestionID };
const matched = { status: "matched", suggestions: [fox], rejected: [wolf], reasons: [] };

function response(payload: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers({ "Content-Type": "application/json" }),
    json: async () => payload,
  } as Response;
}

function recentPage(latestReview?: { id: string; decision: "approved" | "rejected"; comment: string; created_at: string }) {
  return {
    items: [
      { suggestion_id: foxSuggestionID, post_id: postID, post_title: post.title, post_subject: "red fox", subject_confidence: 0.95, image_id: imageID, rank: 1, subject: "red fox", category: "wildlife", caption: "A red fox at the woodland edge.", confidence: 0.94, uncertain: false, similarity: 0.91, accepted: true, rejection_reason: "", created_at: createdAt, latest_review: latestReview, review_count: latestReview ? 1 : 0 },
      { suggestion_id: wolfSuggestionID, post_id: postID, post_title: post.title, post_subject: "red fox", subject_confidence: 0.95, image_id: wolf.image_id, rank: 1, subject: "gray wolf", category: "wildlife", caption: "A gray wolf in a forest.", confidence: 0.96, uncertain: false, similarity: 0.87, accepted: false, rejection_reason: wolf.rejection_reason, created_at: createdAt, review_count: 0 },
    ], total: 2, limit: 50, offset: 0,
  };
}

describe("Image Relevance Studio", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    queryClient.clear();
    window.history.replaceState({}, "", "/");
    fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      if (path.endsWith("/api/health")) return response({ status: "ok" });
      if (path.endsWith("/api/ready")) return response({ status: "ready", checks: { database: "ok" } });
      if (path.includes("/api/v1/images") && init?.method === "POST") {
        return response({ image_id: imageID, job_id: "66666666-6666-4666-8666-666666666666", status: "queued", existing: false }, 202);
      }
      if (path.includes(`/api/v1/images/${imageID}`) && !path.endsWith("/content")) {
        return response({ id: imageID, status: "processed", sha256: "a".repeat(64), mime_type: "image/png", size_bytes: 3, metadata: { subject: "red fox", category: "wildlife", attributes: ["rust coat"], caption: "A red fox.", confidence: 0.94 }, created_at: createdAt });
      }
      if (path.includes("/api/v1/images")) return response(emptyImagePage);
      if (path.includes("/api/v1/matches/") && path.includes("/reviews?")) {
        return response({ items: [{ id: "77777777-7777-4777-8777-777777777777", decision: "approved", comment: "Verified against the article.", created_at: createdAt }], total: 1, limit: 20, offset: 0 });
      }
      if (path.includes("/api/v1/matches/") && path.endsWith("/review") && init?.method === "POST") {
        const review = JSON.parse(String(init.body)) as { decision: "approved" | "rejected"; comment?: string };
        return response({ id: "88888888-8888-4888-8888-888888888888", decision: review.decision, comment: review.comment ?? "", created_at: createdAt }, 201);
      }
      if (path.includes("/api/v1/matches?")) return response(recentPage());
      if (path.endsWith("/api/v1/posts") && init?.method === "POST") return response(post, 201);
      if (path.includes(`/api/v1/posts/${postID}/matches`)) return response(matched);
      return response({ error: { code: "not_found", message: "Not found" } }, 404);
    });
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => queryClient.clear());

  it("navigates to the implemented Matching Studio", async () => {
    render(<App />);
    expect(await screen.findByRole("heading", { name: "Overview" })).toBeTruthy();
    fireEvent.click(screen.getByRole("link", { name: /Matching Studio/ }));
    expect(await screen.findByRole("heading", { name: "Matching Studio" })).toBeTruthy();
    expect(screen.getByLabelText(/Article title/)).toBeTruthy();
    expect(screen.getByLabelText(/Article content/)).toBeTruthy();
  });

  it("creates an article and renders real API-shaped fox and wolf decisions", async () => {
    render(<App />);
    fireEvent.click(screen.getByRole("link", { name: /Matching Studio/ }));
    fireEvent.change(await screen.findByLabelText(/Article title/), { target: { value: post.title } });
    fireEvent.change(screen.getByLabelText(/Article content/), { target: { value: "Red foxes hunt near woodland edges." } });
    fireEvent.click(screen.getByRole("button", { name: "Create article" }));
    await waitFor(() => expect(document.querySelector(".notice-success")?.textContent).toContain("Resolved subject: red fox"));
    fireEvent.click(screen.getByRole("button", { name: /Run matching/ }));
    expect(await screen.findByText("gray wolf")).toBeTruthy();
    expect(screen.getByText(/Subject mismatch: article subject 'red fox'/)).toBeTruthy();
    expect(screen.getByText("Accepted")).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/posts", expect.objectContaining({ method: "POST" }));
    expect(fetchMock).toHaveBeenCalledWith(`/api/v1/posts/${postID}/matches`, expect.objectContaining({ method: "POST" }));
    fireEvent.click(screen.getAllByRole("button", { name: /Inspect decision/ })[1]);
    expect(await screen.findByText("SUBJECT MISMATCH GUARD")).toBeTruthy();
    expect(screen.getByText(/red fox article.*gray wolf image/)).toBeTruthy();
  });

  it("shows the backend no-confident-match response without inventing a recommendation", async () => {
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      if (path.endsWith("/api/health")) return response({ status: "ok" });
      if (path.endsWith("/api/ready")) return response({ status: "ready", checks: { database: "ok" } });
      if (path.endsWith("/api/v1/posts") && init?.method === "POST") return response(post, 201);
      if (path.includes(`/api/v1/posts/${postID}/matches`)) return response({ status: "no_confident_match", suggestions: [], rejected: [], reasons: ["No candidate satisfied the subject and similarity requirements."] });
      return response(emptyImagePage);
    });
    render(<App />);
    fireEvent.click(screen.getByRole("link", { name: /Matching Studio/ }));
    fireEvent.change(await screen.findByLabelText(/Article title/), { target: { value: post.title } });
    fireEvent.change(screen.getByLabelText(/Article content/), { target: { value: "An unrelated article body." } });
    fireEvent.click(screen.getByRole("button", { name: "Create article" }));
    fireEvent.click(await screen.findByRole("button", { name: /Run matching/ }));
    expect(await screen.findByText("NO_CONFIDENT_MATCH")).toBeTruthy();
    expect(screen.getByText("No candidate satisfied the subject and similarity requirements.")).toBeTruthy();
  });

  it("submits review actions and retrieves their persisted audit history", async () => {
    render(<App />);
    fireEvent.click(screen.getByRole("link", { name: "Review Centre" }));
    expect((await screen.findAllByText("Red fox field notes")).length).toBe(2);
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    fireEvent.change(screen.getByLabelText("Optional reviewer note"), { target: { value: "Verified against the article." } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm approved" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(`/api/v1/matches/${foxSuggestionID}/review`, expect.objectContaining({ method: "POST" })));
    expect(await screen.findByText(/Approval saved for this suggestion/)).toBeTruthy();
    fireEvent.click(screen.getAllByRole("button", { name: /History/ })[0]);
    expect(await screen.findByText("Append-only review history")).toBeTruthy();
    expect(await screen.findByText("Verified against the article.")).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledWith(`/api/v1/matches/${foxSuggestionID}/reviews?limit=20&offset=0`, expect.anything());
    fireEvent.click(screen.getAllByRole("button", { name: "Reject" })[1]);
    fireEvent.change(screen.getByLabelText("Optional reviewer note"), { target: { value: "The image is a wolf, not a fox." } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm rejected" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith(`/api/v1/matches/${wolfSuggestionID}/review`, expect.objectContaining({ method: "POST", body: JSON.stringify({ decision: "rejected", comment: "The image is a wolf, not a fox." }) })));
  });

  it("preserves a rejection note after failure and clears it only after a successful retry", async () => {
    let failNextReview = true;
    const originalFetch = fetchMock.getMockImplementation() as (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).endsWith(`/api/v1/matches/${foxSuggestionID}/review`) && failNextReview) {
        failNextReview = false;
        return response({ error: { code: "review_unavailable", message: "The review could not be saved. Try again." } }, 503);
      }
      return originalFetch(input, init);
    });

    render(<App />);
    fireEvent.click(await screen.findByRole("link", { name: "Review Centre" }));
    expect((await screen.findAllByText("Red fox field notes")).length).toBe(2);
    fireEvent.click(screen.getAllByRole("button", { name: "Reject" })[0]);
    const note = screen.getByLabelText("Optional reviewer note") as HTMLTextAreaElement;
    fireEvent.change(note, { target: { value: "The evidence does not support this image." } });
    fireEvent.click(screen.getByRole("button", { name: "Confirm rejected" }));

    expect((await screen.findByRole("alert")).textContent).toContain("The review could not be saved");
    expect((screen.getByLabelText("Optional reviewer note") as HTMLTextAreaElement).value).toBe("The evidence does not support this image.");
    expect((screen.getByRole("button", { name: "Confirm rejected" }) as HTMLButtonElement).disabled).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: "Confirm rejected" }));
    expect(await screen.findByText(/Rejection saved for this suggestion/)).toBeTruthy();
    await waitFor(() => expect(screen.queryByLabelText("Optional reviewer note")).toBeNull());
    expect(fetchMock.mock.calls.filter(([input]) => String(input).endsWith(`/api/v1/matches/${foxSuggestionID}/review`))).toHaveLength(2);
  });

  it("refreshes overview, match history, and open review history after decisions change", async () => {
    const callsTo = (path: string) => fetchMock.mock.calls.filter(([input]) => String(input) === path).length;
    render(<App />);
    await waitFor(() => expect(callsTo("/api/v1/matches?limit=6&offset=0")).toBeGreaterThan(0));

    fireEvent.click(screen.getByRole("link", { name: "Review Centre" }));
    expect((await screen.findAllByText("Red fox field notes")).length).toBe(2);
    await waitFor(() => expect(callsTo("/api/v1/matches?limit=50&offset=0")).toBeGreaterThan(0));

    fireEvent.click(screen.getByRole("link", { name: /Matching Studio/ }));
    fireEvent.change(await screen.findByLabelText(/Article title/), { target: { value: post.title } });
    fireEvent.change(screen.getByLabelText(/Article content/), { target: { value: "Red foxes hunt near woodland edges." } });
    fireEvent.click(screen.getByRole("button", { name: "Create article" }));
    await waitFor(() => expect(document.querySelector(".notice-success")?.textContent).toContain("Resolved subject: red fox"));
    fireEvent.click(screen.getByRole("button", { name: /Run matching/ }));
    expect(await screen.findByText("gray wolf")).toBeTruthy();

    fireEvent.click(screen.getByRole("link", { name: "Overview" }));
    await waitFor(() => expect(callsTo("/api/v1/matches?limit=6&offset=0")).toBeGreaterThan(1));
    fireEvent.click(screen.getByRole("link", { name: "Review Centre" }));
    await waitFor(() => expect(callsTo("/api/v1/matches?limit=50&offset=0")).toBeGreaterThan(1));

    fireEvent.click(screen.getAllByRole("button", { name: /History/ })[0]);
    const historyPath = `/api/v1/matches/${foxSuggestionID}/reviews?limit=20&offset=0`;
    await waitFor(() => expect(callsTo(historyPath)).toBe(1));
    fireEvent.click(screen.getAllByRole("button", { name: "Approve" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "Confirm approved" }));
    expect(await screen.findByText(/Approval saved for this suggestion/)).toBeTruthy();
    await waitFor(() => expect(callsTo(historyPath)).toBeGreaterThan(1));
    fireEvent.click(screen.getByRole("link", { name: "Overview" }));
    await waitFor(() => expect(callsTo("/api/v1/matches?limit=6&offset=0")).toBeGreaterThan(2));
  });

  it("uploads through the API and polls until image metadata is available", async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole("link", { name: "Image Library" }));
    const input = screen.getByLabelText("Choose JPEG or PNG image") as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File(["png"], "fox.png", { type: "image/png" })] } });
    await waitFor(() => expect(screen.getByText("Image processing finished: processed.")).toBeTruthy());
    expect(screen.getByText("red fox")).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/images", expect.objectContaining({ method: "POST" }));
  });

  it("keeps polling both uploads when two images process concurrently", async () => {
    const uploadedIDs = [imageID, secondImageID];
    const reads = new Map<string, number>();
    const originalFetch = fetchMock.getMockImplementation() as (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;
    fetchMock.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const path = String(input);
      if (path === "/api/v1/images" && init?.method === "POST") {
        const id = uploadedIDs.shift();
        if (!id) throw new Error("Unexpected third upload in concurrency test");
        return response({ image_id: id, job_id: "66666666-6666-4666-8666-666666666666", status: "queued", existing: false }, 202);
      }
      const id = [imageID, secondImageID].find((imageIDValue) => path === `/api/v1/images/${imageIDValue}`);
      if (id) {
        const count = (reads.get(id) ?? 0) + 1;
        reads.set(id, count);
        return response({ id, status: count >= 2 ? "processed" : "processing", sha256: "a".repeat(64), mime_type: "image/png", size_bytes: 3, metadata: { subject: "red fox", category: "wildlife", attributes: [], caption: "A red fox.", confidence: 0.94 }, created_at: createdAt });
      }
      return originalFetch(input, init);
    });

    render(<App />);
    fireEvent.click(await screen.findByRole("link", { name: "Image Library" }));
    const input = screen.getByLabelText("Choose JPEG or PNG image") as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File(["one"], "fox-one.png", { type: "image/png" })] } });
    await waitFor(() => expect(fetchMock.mock.calls.filter(([request, init]) => String(request) === "/api/v1/images" && init?.method === "POST")).toHaveLength(1));
    fireEvent.change(input, { target: { files: [new File(["two"], "fox-two.png", { type: "image/png" })] } });
    await waitFor(() => expect(fetchMock.mock.calls.filter(([request, init]) => String(request) === "/api/v1/images" && init?.method === "POST")).toHaveLength(2));

    await waitFor(() => {
      expect(reads.get(imageID)).toBeGreaterThan(1);
      expect(reads.get(secondImageID)).toBeGreaterThan(1);
    }, { timeout: 5000 });
    await waitFor(() => expect(document.querySelectorAll(".tracker-line")).toHaveLength(0), { timeout: 5000 });
    await new Promise((resolve) => setTimeout(resolve, 1700));
    const settledReads = new Map(reads);
    await new Promise((resolve) => setTimeout(resolve, 1700));
    expect(reads).toEqual(settledReads);
  }, 15000);
});
