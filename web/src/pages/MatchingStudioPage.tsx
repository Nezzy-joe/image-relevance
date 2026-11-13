import { useMutation, useQueryClient } from "@tanstack/react-query";
import { AlertCircle, ArrowRight, BookOpenText, Check, ChevronRight, FileText, LoaderCircle, RotateCcw, ScanEye, Sparkles, X } from "lucide-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, Card, CardHeading, SectionEyebrow, StatusPill } from "../components/ui/primitives";
import { ApiError, api } from "../lib/api";
import { queryKeys } from "../lib/query-keys";
import type { MatchCandidate, MatchResult, Post } from "../types/api";

type ArticleTemplate = { label: string; title: string; body: string; icon: "field" | "guide" | "nature" };

const templates: ArticleTemplate[] = [
  {
    label: "Wildlife field note",
    title: "Red foxes adapt to changing woodland edges",
    body: "Red foxes are highly adaptable mammals that hunt small animals along woodland boundaries. Their rust-coloured coats, narrow muzzles, and bushy tails help distinguish them from other canids. This field note describes how foxes use cover and quiet paths while searching for prey.",
    icon: "field",
  },
  {
    label: "Species guide",
    title: "How gray wolves communicate across a pack",
    body: "Gray wolves use vocal signals, scent, posture, and coordinated movement to communicate across their range. Pack members travel through forest and open country while maintaining close social bonds. This guide focuses on wolf behaviour and the features that set wolves apart from foxes.",
    icon: "guide",
  },
  {
    label: "Nature story",
    title: "A quiet morning in the pollinator garden",
    body: "A healthy garden supports bees, butterflies, and other pollinators throughout the growing season. Diverse flowers provide nectar and pollen, while native plants offer shelter. This article follows the small changes that make an urban garden more welcoming to pollinating insects.",
    icon: "nature",
  },
];

function errorText(error: unknown) {
  return error instanceof ApiError ? error.message : "The request could not be completed. Try again.";
}

function byteLength(value: string) {
  return new TextEncoder().encode(value).length;
}

function percent(value: number) {
  return `${Math.round(value * 100)}%`;
}

function CandidateCard({
  candidate,
  kind,
  onInspect,
}: {
  candidate: MatchCandidate;
  kind: "accepted" | "rejected";
  onInspect: () => void;
}) {
  return (
    <Card className={`candidate-card candidate-${kind}`}>
      <div className="candidate-topline">
        <div className="candidate-rank">#{candidate.rank}</div>
        <StatusPill status={candidate.accepted ? "accepted" : "rejected"}>
          {candidate.accepted ? "Accepted" : "Rejected"}
        </StatusPill>
      </div>
      <div className="candidate-subject-row">
        <div className={`decision-symbol ${candidate.accepted ? "decision-symbol-yes" : "decision-symbol-no"}`} aria-hidden="true">
          {candidate.accepted ? <Check size={17} /> : <X size={17} />}
        </div>
        <div className="candidate-subject-copy">
          <strong>{candidate.subject}</strong>
          <span>{candidate.category}</span>
        </div>
        <div className="candidate-score"><strong>{candidate.similarity.toFixed(3)}</strong><span>similarity</span></div>
      </div>
      <div className="candidate-metrics">
        <span><i />Classification confidence <strong>{percent(candidate.confidence)}</strong></span>
        <span><i />Subject guard <strong>{candidate.accepted ? "compatible" : "blocked"}</strong></span>
      </div>
      {!candidate.accepted && candidate.rejection_reason && (
        <div className="rejection-explanation"><AlertCircle size={15} /><p>{candidate.rejection_reason}</p></div>
      )}
      <Button
        variant="secondary"
        size="sm"
        className="candidate-inspect"
        onClick={onInspect}
        disabled={!candidate.suggestion_id}
        title={candidate.suggestion_id ? "Open the persisted decision details" : "Decision details are not available yet"}
      >
        <ScanEye size={14} />Inspect decision<ChevronRight size={14} />
      </Button>
    </Card>
  );
}

export function MatchingStudioPage() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [createdPost, setCreatedPost] = useState<Post | null>(null);
  const [matchResult, setMatchResult] = useState<MatchResult | null>(null);
  const [validation, setValidation] = useState<string | null>(null);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const createMutation = useMutation({
    mutationFn: api.createPost,
    onSuccess: (post) => {
      setCreatedPost(post);
      setMatchResult(null);
      setValidation(null);
    },
  });
  const matchMutation = useMutation({
    mutationFn: api.matchPost,
    onSuccess: (result) => {
      setMatchResult(result);
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.recentMatches }),
        queryClient.invalidateQueries({ queryKey: queryKeys.matchHistory }),
      ]);
    },
  });

  const accepted = matchResult?.suggestions ?? [];
  const rejected = matchResult?.rejected ?? [];
  const characters = Array.from(body).length;
  const bytes = byteLength(body);
  const isNoMatch = matchResult?.status === "no_confident_match";

  function reset() {
    setTitle("");
    setBody("");
    setCreatedPost(null);
    setMatchResult(null);
    setValidation(null);
    createMutation.reset();
    matchMutation.reset();
  }

  function applyTemplate(template: ArticleTemplate) {
    setTitle(template.title);
    setBody(template.body);
    setCreatedPost(null);
    setMatchResult(null);
    setValidation(null);
    createMutation.reset();
    matchMutation.reset();
  }

  function editTitle(value: string) {
    setTitle(value);
    setCreatedPost(null);
    setMatchResult(null);
    createMutation.reset();
    matchMutation.reset();
  }

  function editBody(value: string) {
    setBody(value);
    setCreatedPost(null);
    setMatchResult(null);
    createMutation.reset();
    matchMutation.reset();
  }

  function createPost(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedTitle = title.trim();
    const trimmedBody = body.trim();
    if (!trimmedTitle || !trimmedBody) {
      setValidation("Add an article title and body before creating the article.");
      return;
    }
    if (byteLength(trimmedTitle) > 200 || byteLength(trimmedBody) > 20_000) {
      setValidation("The API limit is 200 UTF-8 bytes for the title and 20,000 UTF-8 bytes for the body.");
      return;
    }
    setValidation(null);
    createMutation.mutate({ title: trimmedTitle, body: trimmedBody });
  }

  function inspect(candidate: MatchCandidate) {
    if (!createdPost || !matchResult) return;
    navigate("/decisions", { state: { kind: "live", post: createdPost, body, result: matchResult, candidate } });
  }

  return (
    <div className="page-stack studio-stack">
      <section className="page-heading">
        <div>
          <SectionEyebrow>MATCHING WORKSPACE <span /> LIVE API</SectionEyebrow>
          <h1>Matching Studio</h1>
          <p>Resolve an article’s subject, compare it with processed images, and inspect the decisions returned by the matching engine.</p>
        </div>
        <div className="studio-provider-chip"><Sparkles size={14} /><span>Gemini + pgvector</span></div>
      </section>

      <div className="studio-layout">
        <section className="studio-editor-column">
          <Card className="article-editor-card">
            <div className="studio-card-heading">
              <div className="studio-step">01</div>
              <div><SectionEyebrow>ARTICLE INPUT</SectionEyebrow><CardHeading>Compose your article</CardHeading></div>
              <span className="studio-card-hint">Only real API results are shown</span>
            </div>
            <div className="template-row" aria-label="Sample article templates">
              <span className="template-label">START WITH A SAMPLE</span>
              {templates.map((template) => (
                <button className="template-button" key={template.label} type="button" onClick={() => applyTemplate(template)}>
                  {template.icon === "field" ? <FileText size={13} /> : template.icon === "guide" ? <BookOpenText size={13} /> : <Sparkles size={13} />}
                  {template.label}
                </button>
              ))}
            </div>

            <form className="article-form" onSubmit={createPost} noValidate>
              <label className="studio-label" htmlFor="article-title">Article title <span>Required · up to 200 bytes</span></label>
              <input
                id="article-title"
                className="studio-input title-input"
                value={title}
                onChange={(event) => editTitle(event.currentTarget.value)}
                maxLength={200}
                placeholder="e.g. Red foxes adapt to changing woodland edges"
                autoComplete="off"
              />
              <div className="editor-label-row">
                <label className="studio-label" htmlFor="article-body">Article content <span>Required · up to 20,000 bytes</span></label>
                <span className={`character-count${bytes > 20_000 ? " character-over" : ""}`}>{characters.toLocaleString()} characters</span>
              </div>
              <textarea
                id="article-body"
                className="studio-input article-textarea"
                value={body}
                onChange={(event) => editBody(event.currentTarget.value)}
                placeholder="Paste an article or write a short sample. The matching service resolves the main subject from this text."
                rows={12}
              />
              <div className="editor-meta"><span>{bytes.toLocaleString()} / 20,000 UTF-8 bytes</span><span>Article text is sent to the local API for subject resolution and embedding.</span></div>

              {(validation || createMutation.isError) && <div className="notice notice-error" role="alert"><span><AlertCircle size={15} />{validation ?? errorText(createMutation.error)}</span>{validation && <button aria-label="Dismiss validation message" onClick={() => setValidation(null)}><X size={15} /></button>}</div>}
              {createMutation.isSuccess && createdPost && (
                <div className="notice notice-success" role="status"><span><Check size={15} />Article saved. Resolved subject: <strong>{createdPost.subject}</strong> · {percent(createdPost.subject_confidence)} confidence.</span></div>
              )}

              <div className="editor-actions">
                <Button variant="ghost" onClick={reset} disabled={createMutation.isPending || matchMutation.isPending}><RotateCcw size={15} />Clear</Button>
                <Button variant="primary" type="submit" disabled={createMutation.isPending || matchMutation.isPending}>
                  {createMutation.isPending ? <LoaderCircle size={15} className="spin" /> : <FileText size={15} />}
                  {createMutation.isPending ? "Creating article…" : "Create article"}
                </Button>
              </div>
            </form>
          </Card>

          <Card className="templates-note"><div className="templates-note-icon"><BookOpenText size={17} /></div><div><strong>Samples are just starting points</strong><p>Templates fill the editor only. Recommendations and decisions below always come from the API response for the article you submit.</p></div></Card>
        </section>

        <aside className="match-run-column">
          <Card className="run-match-card">
            <div className="studio-step studio-step-glow">02</div>
            <SectionEyebrow>MATCH RUN</SectionEyebrow>
            <CardHeading>Find relevant images</CardHeading>
            <p className="run-match-copy">Create the article first. Then the service searches tenant-scoped image embeddings and applies the subject mismatch guard.</p>
            {createdPost ? (
              <div className="resolved-article">
                <span>ARTICLE READY</span><strong>{createdPost.title}</strong>
                <div><span>Resolved subject</span><b>{createdPost.subject}</b></div>
                <div className="confidence-meter"><span style={{ width: `${createdPost.subject_confidence * 100}%` }} /></div>
                <small>{percent(createdPost.subject_confidence)} subject confidence</small>
              </div>
            ) : <div className="run-empty"><div><FileText size={17} /></div><span>Your created article will appear here.</span></div>}
            {matchMutation.isError && <div className="notice notice-error" role="alert"><span><AlertCircle size={15} />{errorText(matchMutation.error)}</span></div>}
            <Button variant="primary" className="run-match-button" disabled={!createdPost || matchMutation.isPending} onClick={() => createdPost && matchMutation.mutate(createdPost.id)}>
              {matchMutation.isPending ? <LoaderCircle size={16} className="spin" /> : <Sparkles size={15} />}
              {matchMutation.isPending ? "Matching images…" : "Run matching"}<ArrowRight size={15} />
            </Button>
            <div className="run-match-foot"><span /><span>Uses the configured local API and database</span></div>
          </Card>
          <Card className="guard-note"><div><div className="guard-orb"><Check size={16} /></div><div><strong>Mismatch guard active</strong><p>Subject compatibility and confidence thresholds are applied by the backend before any candidate is accepted.</p></div></div><StatusPill status="protected">GUARDED</StatusPill></Card>
        </aside>
      </div>

      {matchResult && createdPost && (
        <section className="match-results-section" aria-live="polite" aria-busy={matchMutation.isPending}>
          <div className="results-heading match-results-heading">
            <div><SectionEyebrow>ENGINE RESPONSE <span /> ARTICLE MATCH</SectionEyebrow><h2>Candidate decisions</h2><p>“{createdPost.title}” · subject <strong>{createdPost.subject}</strong></p></div>
            <div className="result-counts"><span className="result-count accepted-count"><Check size={13} />{accepted.length} accepted</span><span className="result-count rejected-count"><X size={13} />{rejected.length} rejected</span></div>
          </div>

          {isNoMatch && (
            <Card className="no-match-state" role="status"><div className="no-match-mark"><AlertCircle size={20} /></div><div><StatusPill status="no-confident-match">NO_CONFIDENT_MATCH</StatusPill><h3>No candidate passed the confidence checks</h3><p>{matchResult.reasons[0] ?? "The backend did not find a candidate that met its acceptance requirements."}</p></div></Card>
          )}
          {!accepted.length && !rejected.length && !isNoMatch && (
            <Card className="no-match-state" role="status"><div className="no-match-mark"><AlertCircle size={20} /></div><div><StatusPill status="empty">NO CANDIDATES</StatusPill><h3>No eligible image candidates were returned</h3><p>The API returned an empty candidate set. Add processed images to the development tenant and run matching again.</p></div></Card>
          )}

          {accepted.length > 0 && <div className="candidate-group"><div className="candidate-group-heading"><h3><Check size={15} />Recommended images</h3><span>{accepted.length} returned by the backend</span></div><div className="candidate-grid">{accepted.map((candidate) => <CandidateCard key={candidate.suggestion_id ?? candidate.image_id} candidate={candidate} kind="accepted" onInspect={() => inspect(candidate)} />)}</div></div>}
          {rejected.length > 0 && <div className="candidate-group"><div className="candidate-group-heading"><h3><X size={15} />Rejected candidates</h3><span>Guard explanations come directly from the API</span></div><div className="candidate-grid">{rejected.map((candidate) => <CandidateCard key={candidate.suggestion_id ?? candidate.image_id} candidate={candidate} kind="rejected" onInspect={() => inspect(candidate)} />)}</div></div>}
          <div className="result-provenance"><span /><span>Live response from <code>POST /api/v1/posts/{"{postID}"}/matches</code></span><Button variant="ghost" size="sm" onClick={() => matchMutation.mutate(createdPost.id)} disabled={matchMutation.isPending}>Run again<ChevronRight size={13} /></Button></div>
        </section>
      )}
    </div>
  );
}
