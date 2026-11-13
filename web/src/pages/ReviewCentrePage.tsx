import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertCircle, ArrowRight, Check, ChevronDown, ChevronRight, Clock3, History, ImageOff, LoaderCircle, MessageSquareText, RotateCcw, ShieldCheck, X } from "lucide-react";
import { useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, Card, SectionEyebrow, Skeleton, StatusPill } from "../components/ui/primitives";
import { ApiError, api } from "../lib/api";
import { queryKeys } from "../lib/query-keys";
import type { RecentMatch, ReviewAction } from "../types/api";

const PAGE_SIZE = 50;

function errorText(error: unknown) {
  return error instanceof ApiError ? error.message : "The request could not be completed. Try again.";
}

function dateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

function ReviewHistory({ suggestionID, open }: { suggestionID: string; open: boolean }) {
  const query = useQuery({
    queryKey: queryKeys.reviewHistory(suggestionID),
    queryFn: () => api.listReviewHistory(suggestionID),
    enabled: open,
  });
  if (!open) return null;
  return (
    <div className="review-history-panel">
      <div className="review-history-heading"><History size={15} /><strong>Append-only review history</strong><span>{query.data ? `${query.data.total} action${query.data.total === 1 ? "" : "s"}` : "Audit trail"}</span></div>
      {query.isLoading && <div className="history-loading"><LoaderCircle size={15} className="spin" />Loading persisted audit history…</div>}
      {query.isError && <div className="inline-alert compact-alert" role="alert"><AlertCircle size={15} /><span>{errorText(query.error)}</span><Button variant="ghost" size="sm" onClick={() => void query.refetch()}>Retry</Button></div>}
      {query.data?.items.length === 0 && <p className="history-empty">No reviewer action has been recorded for this suggestion.</p>}
      {query.data?.items.map((action: ReviewAction) => (
        <div className="review-history-item" key={action.id}>
          <span className={`history-decision-dot history-${action.decision}`} />
          <div><strong>{action.decision === "approved" ? "Approved" : "Rejected"}</strong>{action.comment && <p>{action.comment}</p>}<span>{dateTime(action.created_at)}</span></div>
        </div>
      ))}
      <p className="reviewer-identity-note">Reviewer identity is not authenticated or recorded in this development build.</p>
    </div>
  );
}

function ReviewCard({
  item,
  onInspect,
  onReview,
  pending,
  activeHistoryID,
  onToggleHistory,
}: {
  item: RecentMatch;
  onInspect: () => void;
  onReview: (suggestionID: string, decision: "approved" | "rejected", comment: string) => Promise<void>;
  pending: boolean;
  activeHistoryID: string | null;
  onToggleHistory: (suggestionID: string) => void;
}) {
  const [decision, setDecision] = useState<"approved" | "rejected" | null>(null);
  const [comment, setComment] = useState("");
  const [previewFailed, setPreviewFailed] = useState(false);
  const submitting = useRef(false);
  const reviewed = item.review_count > 0;
  const isOpen = activeHistoryID === item.suggestion_id;

  function startReview(next: "approved" | "rejected") {
    setDecision(next);
    setComment("");
  }

  async function submitReview() {
    if (!decision || pending || submitting.current) return;
    submitting.current = true;
    try {
      await onReview(item.suggestion_id, decision, comment.trim());
      setDecision(null);
      setComment("");
    } catch {
      // Keep the action and note available so the user can retry.
    } finally {
      submitting.current = false;
    }
  }

  return (
    <Card className="review-card">
      <div className="review-card-main">
        <button type="button" className="review-image-button" onClick={onInspect} aria-label={`Inspect ${item.subject || "image"} decision`}>
          {previewFailed ? <div className="image-fallback"><ImageOff size={20} /><span>Preview unavailable</span></div> : <img src={api.imageContentURL(item.image_id)} alt={item.caption || `${item.subject || "Candidate"} image`} onError={() => setPreviewFailed(true)} />}
          <span className="review-image-open"><ArrowRight size={14} /></span>
        </button>
        <div className="review-card-content">
          <div className="review-article-title-row"><div><SectionEyebrow>ARTICLE</SectionEyebrow><h3>{item.post_title}</h3></div><StatusPill status={item.accepted ? "accepted" : "rejected"}>{item.accepted ? "AI accepted" : "AI rejected"}</StatusPill></div>
          <p className="review-subject-line"><span>{item.post_subject}</span><i />{item.subject || "Unknown image subject"}<i />{item.category || "Uncategorized"}</p>
          {item.caption && <p className="review-caption">{item.caption}</p>}
          <div className="review-evidence-row"><span>Similarity <strong>{item.similarity.toFixed(3)}</strong></span><span>Image confidence <strong>{Math.round(item.confidence * 100)}%</strong></span><span>Decision time <strong>{dateTime(item.created_at)}</strong></span></div>
          {!item.accepted && item.rejection_reason && <p className="review-rejection"><X size={13} />{item.rejection_reason}</p>}
          <div className="review-review-state"><span className="review-state-icon">{reviewed ? <Check size={13} /> : <Clock3 size={13} />}</span><span>{reviewed ? `Latest review: ${item.latest_review?.decision ?? "recorded"}` : "Awaiting human review"}</span>{item.latest_review && <time dateTime={item.latest_review.created_at}>{dateTime(item.latest_review.created_at)}</time>}<span className="review-action-count">{item.review_count} action{item.review_count === 1 ? "" : "s"}</span></div>
        </div>
      </div>

      <div className="review-card-actions">
        <Button variant="ghost" size="sm" onClick={onInspect}><ShieldCheck size={14} />Inspect decision</Button>
        <div className="review-action-buttons">
          <Button variant="secondary" size="sm" onClick={() => startReview("approved")} disabled={pending}><Check size={14} />Approve</Button>
          <Button variant="danger" size="sm" onClick={() => startReview("rejected")} disabled={pending}><X size={14} />Reject</Button>
        </div>
        <Button variant="ghost" size="sm" onClick={() => onToggleHistory(item.suggestion_id)} aria-expanded={isOpen}><History size={14} />History ({item.review_count}){isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</Button>
      </div>

      {decision && <div className={`review-compose review-compose-${decision}`}>
        <div className="review-compose-title"><MessageSquareText size={15} /><strong>{decision === "approved" ? "Approve this suggestion" : "Reject this suggestion"}</strong><span>Optional reviewer note · max 2,000 characters</span></div>
        <textarea value={comment} maxLength={2000} rows={2} onChange={(event) => setComment(event.currentTarget.value)} placeholder="Add context for the audit history (optional)" aria-label="Optional reviewer note" />
        <div className="review-compose-actions"><span>{comment.length.toLocaleString()} / 2,000</span><Button variant="ghost" size="sm" onClick={() => setDecision(null)} disabled={pending}>Cancel</Button><Button variant={decision === "approved" ? "primary" : "danger"} size="sm" onClick={() => { void submitReview(); }} disabled={pending}>{pending ? <LoaderCircle size={14} className="spin" /> : decision === "approved" ? <Check size={14} /> : <X size={14} />}{pending ? "Saving…" : `Confirm ${decision}`}</Button></div>
      </div>}

      <ReviewHistory suggestionID={item.suggestion_id} open={isOpen} />
    </Card>
  );
}

export function ReviewCentrePage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [decisionFilter, setDecisionFilter] = useState<"all" | "accepted" | "rejected">("all");
  const [reviewFilter, setReviewFilter] = useState<"all" | "unreviewed" | "approved" | "rejected">("all");
  const [activeHistoryID, setActiveHistoryID] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<string | null>(null);
  const historyQuery = useInfiniteQuery({
    queryKey: queryKeys.matchHistory,
    queryFn: ({ pageParam }) => api.listMatches(PAGE_SIZE, pageParam),
    initialPageParam: 0,
    getNextPageParam: (lastPage, pages) => {
      const loaded = pages.reduce((sum, page) => sum + page.items.length, 0);
      return loaded < lastPage.total ? loaded : undefined;
    },
  });
  const reviewMutation = useMutation({
    mutationFn: ({ suggestionID, decision, comment }: { suggestionID: string; decision: "approved" | "rejected"; comment: string }) =>
      api.submitReview(suggestionID, { decision, ...(comment ? { comment } : {}) }),
    onSuccess: (result, variables) => {
      setActionError(null);
      setActionSuccess(`${result.decision === "approved" ? "Approval" : "Rejection"} saved for this suggestion. The original AI decision is unchanged.`);
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.matchHistory }),
        queryClient.invalidateQueries({ queryKey: queryKeys.recentMatches }),
        queryClient.invalidateQueries({ queryKey: queryKeys.reviewHistory(variables.suggestionID) }),
      ]);
    },
    onError: (error) => setActionError(errorText(error)),
  });

  const items = historyQuery.data?.pages.flatMap((page) => page.items) ?? [];
  const total = historyQuery.data?.pages[0]?.total ?? 0;
  const filtered = items.filter((item) => {
    const matchesDecision = decisionFilter === "all" || (decisionFilter === "accepted" ? item.accepted : !item.accepted);
    const latest = item.latest_review?.decision;
    const matchesReview = reviewFilter === "all" || (reviewFilter === "unreviewed" ? item.review_count === 0 : latest === reviewFilter);
    return matchesDecision && matchesReview;
  });

  function inspect(item: RecentMatch) {
    navigate("/decisions", { state: { kind: "history", match: item } });
  }

  function submitReview(suggestionID: string, decision: "approved" | "rejected", comment: string) {
    setActionError(null);
    setActionSuccess(null);
    return reviewMutation.mutateAsync({ suggestionID, decision, comment }).then(() => undefined);
  }

  return (
    <div className="page-stack review-centre-stack">
      <section className="page-heading review-heading">
        <div><SectionEyebrow>HUMAN OVERSIGHT <span /> PERSISTED HISTORY</SectionEyebrow><h1>Review Centre</h1><p>Audit original AI decisions, add a human review action, and inspect the append-only history for each suggestion.</p></div>
        <div className="review-total-chip"><History size={15} /><strong>{historyQuery.data ? total.toLocaleString() : "—"}</strong><span>persisted decisions</span></div>
      </section>

      <Card className="review-integrity-note"><div className="integrity-note-icon"><ShieldCheck size={17} /></div><div><strong>AI decision and human review are separate records</strong><p>Approve/reject actions append to the audit trail. They never overwrite the original accepted flag or rejection reason. Reviewer identity authentication is not implemented.</p></div><StatusPill status="append-only">APPEND-ONLY</StatusPill></Card>

      <div className="review-toolbar">
        <div className="review-toolbar-copy"><SectionEyebrow>RECENT DECISIONS</SectionEyebrow><strong>{historyQuery.data ? `${filtered.length} shown from ${items.length} loaded` : "Loading persisted records"}</strong></div>
        <div className="review-filters">
          <label><span>AI decision</span><select value={decisionFilter} onChange={(event) => setDecisionFilter(event.currentTarget.value as typeof decisionFilter)}><option value="all">All decisions</option><option value="accepted">AI accepted</option><option value="rejected">AI rejected</option></select></label>
          <label><span>Review status</span><select value={reviewFilter} onChange={(event) => setReviewFilter(event.currentTarget.value as typeof reviewFilter)}><option value="all">Any review status</option><option value="unreviewed">Awaiting review</option><option value="approved">Latest review approved</option><option value="rejected">Latest review rejected</option></select></label>
        </div>
      </div>

      {actionError && <div className="inline-alert" role="alert"><AlertCircle size={17} /><span>{actionError}</span><Button variant="ghost" size="sm" onClick={() => setActionError(null)}>Dismiss</Button></div>}
      {actionSuccess && <div className="notice notice-success" role="status"><span><Check size={15} />{actionSuccess}</span><button aria-label="Dismiss review confirmation" onClick={() => setActionSuccess(null)}><X size={15} /></button></div>}
      {historyQuery.isError && <div className="inline-alert" role="alert"><AlertCircle size={17} /><span>{errorText(historyQuery.error)} Persisted review data is unavailable until the local API and database are ready.</span><Button variant="ghost" size="sm" onClick={() => void historyQuery.refetch()}>Retry</Button></div>}

      {historyQuery.isLoading ? <div className="review-list" aria-busy="true">{Array.from({ length: 3 }, (_, index) => <Card className="review-skeleton" key={index}><Skeleton className="skeleton-review-image" /><div><Skeleton className="skeleton-line" /><Skeleton className="skeleton-line short" /><Skeleton className="skeleton-line" /></div></Card>)}</div> : filtered.length ? (
        <div className="review-list">{filtered.map((item) => <ReviewCard key={item.suggestion_id} item={item} onInspect={() => inspect(item)} onReview={submitReview} pending={reviewMutation.isPending && reviewMutation.variables?.suggestionID === item.suggestion_id} activeHistoryID={activeHistoryID} onToggleHistory={(id) => setActiveHistoryID((current) => current === id ? null : id)} />)}</div>
      ) : !historyQuery.isError && (
        <Card className="review-empty"><div className="review-empty-icon"><History size={22} /></div><h2>{total === 0 ? "No matching decisions yet" : "No decisions match these filters"}</h2><p>{total === 0 ? "Run an article match in Matching Studio. Persisted accepted and rejected candidates will appear here." : "Change the AI decision or review status filters to see more records."}</p>{total === 0 && <Button variant="primary" onClick={() => navigate("/matching")}>Open Matching Studio<ArrowRight size={15} /></Button>}{total > 0 && <Button variant="secondary" onClick={() => { setDecisionFilter("all"); setReviewFilter("all"); }}><RotateCcw size={14} />Clear filters</Button>}</Card>
      )}

      {historyQuery.hasNextPage && <div className="review-load-more"><span>Showing the newest {items.length} of {total.toLocaleString()} records</span><Button variant="secondary" onClick={() => void historyQuery.fetchNextPage()} disabled={historyQuery.isFetchingNextPage}>{historyQuery.isFetchingNextPage ? <LoaderCircle size={14} className="spin" /> : <ChevronDown size={14} />}Load more decisions</Button></div>}
      {historyQuery.data && !historyQuery.hasNextPage && total > 0 && <div className="result-provenance"><span /><span>Loaded {items.length} persisted decision record{items.length === 1 ? "" : "s"} · current tenant scope: development workspace</span></div>}
    </div>
  );
}
