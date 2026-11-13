import { AlertTriangle, ArrowLeft, ArrowRight, Check, ImageOff, ShieldCheck, X } from "lucide-react";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Button, Card, CardHeading, SectionEyebrow, StatusPill } from "../components/ui/primitives";
import { api } from "../lib/api";
import type { MatchCandidate, MatchResult, Post, RecentMatch } from "../types/api";

type LiveInspection = { kind: "live"; post: Post; body: string; result: MatchResult; candidate: MatchCandidate };
type HistoryInspection = { kind: "history"; match: RecentMatch };
type Inspection = LiveInspection | HistoryInspection;

function confidence(value: number) {
  return `${Math.round(value * 100)}%`;
}

function ImageEvidence({ imageID, alt }: { imageID: string; alt: string }) {
  const [failed, setFailed] = useState(false);
  return (
    <div className="inspection-image-frame">
      {failed ? <div className="image-fallback"><ImageOff size={25} /><span>Image preview unavailable</span></div> : <img src={api.imageContentURL(imageID)} alt={alt} onError={() => setFailed(true)} />}
      <span className="inspection-image-label"><span />STORED IMAGE</span>
    </div>
  );
}

export function DecisionInspectorPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const inspection = location.state as Inspection | null;

  if (!inspection || (inspection.kind !== "live" && inspection.kind !== "history")) {
    return (
      <div className="page-stack">
        <section className="page-heading"><div><SectionEyebrow>DECISION TRACE</SectionEyebrow><h1>Decision Inspector</h1><p>Inspect the article, image metadata, similarity, and exact guard decision together.</p></div></section>
        <Card className="inspector-empty"><div className="inspector-empty-icon"><ShieldCheck size={23} /></div><StatusPill status="ready">READY FOR A DECISION</StatusPill><h2>Open an actual matching decision</h2><p>Run a match in Matching Studio, then choose <strong>Inspect decision</strong> on any accepted or rejected candidate. Review Centre decisions can also open here.</p><Button variant="primary" onClick={() => navigate("/matching")}>Go to Matching Studio<ArrowRight size={15} /></Button></Card>
      </div>
    );
  }

  const history = inspection.kind === "history" ? inspection.match : null;
  const candidate = inspection.kind === "live" ? inspection.candidate : null;
  const accepted = candidate?.accepted ?? history?.accepted ?? false;
  const reason = candidate?.rejection_reason ?? history?.rejection_reason ?? "";
  const imageID = candidate?.image_id ?? history?.image_id ?? "";
  const imageSubject = candidate?.subject ?? history?.subject ?? "Unknown subject";
  const imageCategory = candidate?.category ?? history?.category ?? "Uncategorized";
  const similarity = candidate?.similarity ?? history?.similarity ?? 0;
  const imageConfidence = candidate?.confidence ?? history?.confidence ?? 0;
  const uncertain = candidate?.uncertain ?? history?.uncertain ?? false;
  const postTitle = inspection.kind === "live" ? inspection.post.title : inspection.match.post_title;
  const postSubject = inspection.kind === "live" ? inspection.post.subject : inspection.match.post_subject;
  const postConfidence = inspection.kind === "live" ? inspection.post.subject_confidence : inspection.match.subject_confidence;

  return (
    <div className="page-stack inspector-stack">
      <section className="page-heading inspector-heading">
        <div><SectionEyebrow>DECISION TRACE <span /> {inspection.kind === "live" ? "LIVE MATCH RUN" : "PERSISTED HISTORY"}</SectionEyebrow><h1>Decision Inspector</h1><p>One candidate, all the evidence, and the backend’s original decision.</p></div>
        <Button variant="secondary" onClick={() => navigate(-1)}><ArrowLeft size={15} />Back to results</Button>
      </section>

      <Card className={`decision-banner ${accepted ? "decision-banner-accepted" : "decision-banner-rejected"}`}>
        <div className="decision-banner-symbol">{accepted ? <Check size={20} /> : <X size={20} />}</div>
        <div className="decision-banner-copy"><SectionEyebrow>ORIGINAL AI DECISION</SectionEyebrow><h2>{accepted ? "Candidate accepted" : "Candidate rejected"}</h2><p>{accepted ? "The candidate passed the backend subject and confidence guards." : reason || "The backend rejected this candidate."}</p></div>
        <StatusPill status={accepted ? "accepted" : "rejected"}>{accepted ? "ACCEPTED" : "REJECTED"}</StatusPill>
      </Card>

      {!accepted && /subject mismatch/i.test(reason) && (
        <Card className="mismatch-callout"><div className="mismatch-icon"><AlertTriangle size={19} /></div><div><span>SUBJECT MISMATCH GUARD</span><strong>{postSubject} article <i>↔</i> {imageSubject} image</strong><p>The species subjects do not match. This image remains rejected even if its vector similarity is high.</p></div><span className="mismatch-stop">BLOCKED</span></Card>
      )}

      <div className="inspection-columns">
        <Card className="inspection-article-card">
          <div className="inspection-card-heading"><div className="inspection-step">A</div><div><SectionEyebrow>SOURCE ARTICLE</SectionEyebrow><CardHeading>{postTitle}</CardHeading></div></div>
          <div className="inspection-subject-block"><span>Resolved article subject</span><strong>{postSubject}</strong><div className="confidence-meter"><span style={{ width: `${postConfidence * 100}%` }} /></div><small>{confidence(postConfidence)} subject confidence</small></div>
          {inspection.kind === "live" ? <div className="article-excerpt"><span>ARTICLE TEXT</span><p>{inspection.body}</p></div> : <div className="history-note"><span>Article body is not part of the recent-match API response. Subject and title above are stored article fields.</span></div>}
          {inspection.kind === "history" && <div className="inspection-record-id"><span>POST ID</span><code>{inspection.match.post_id}</code></div>}
        </Card>

        <div className="inspection-connector" aria-hidden="true"><span><ArrowRight size={17} /></span><small>COMPARED</small></div>

        <Card className="inspection-image-card">
          <div className="inspection-card-heading"><div className="inspection-step image-step">B</div><div><SectionEyebrow>CANDIDATE IMAGE</SectionEyebrow><CardHeading>{imageSubject}</CardHeading></div></div>
          <ImageEvidence imageID={imageID} alt={`${imageSubject} image candidate`} />
          <div className="inspection-meta-grid"><div><span>Detected subject</span><strong>{imageSubject}</strong></div><div><span>Category</span><strong>{imageCategory}</strong></div><div><span>Classification confidence</span><strong>{confidence(imageConfidence)}{uncertain ? " · uncertain" : ""}</strong></div><div><span>Cosine similarity</span><strong>{similarity.toFixed(4)}</strong></div><div><span>Candidate rank</span><strong>#{candidate?.rank ?? history?.rank ?? "—"}</strong></div><div><span>Suggestion ID</span><code>{candidate?.suggestion_id ?? history?.suggestion_id ?? "Not persisted"}</code></div></div>
          {history?.caption && <div className="inspection-caption"><span>Stored caption</span><p>{history.caption}</p></div>}
        </Card>
      </div>

      <Card className="inspector-explanation">
        <div className="explanation-icon">{accepted ? <ShieldCheck size={18} /> : <AlertTriangle size={18} />}</div>
        <div><SectionEyebrow>BACKEND EXPLANATION</SectionEyebrow><CardHeading>{accepted ? "Why this candidate passed" : "Why this candidate was rejected"}</CardHeading><p>{reason || "No rejection explanation was returned because the candidate passed the backend decision rules."}</p></div>
        {inspection.kind === "history" && <Button variant="secondary" size="sm" onClick={() => navigate("/reviews")}>Review history<ArrowRight size={13} /></Button>}
      </Card>
      <div className="result-provenance"><span /><span>{inspection.kind === "live" ? `Actual response for post ${inspection.post.id}` : `Persisted suggestion ${inspection.match.suggestion_id}`}</span></div>
    </div>
  );
}
