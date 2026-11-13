import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  CheckCircle2,
  Clock3,
  Database,
  FileImage,
  ImagePlus,
  RefreshCw,
  Server,
  ShieldAlert,
  Sparkles,
  XCircle,
} from "lucide-react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { queryKeys } from "../lib/query-keys";
import type { RecentMatch } from "../types/api";
import { Button, Card, CardHeading, SectionEyebrow, Skeleton, StatusPill } from "../components/ui/primitives";

const formatDate = (value: string) => new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }).format(new Date(value));

function MetricCard({ label, value, hint, icon: Icon, tone = "default", href }: {
  label: string; value: number | undefined; hint: string; icon: typeof FileImage; tone?: string; href?: string;
}) {
  const body = (
    <>
      <div className="metric-top"><span className="metric-label">{label}</span><span className={`metric-icon metric-${tone}`}><Icon size={17} strokeWidth={1.8} /></span></div>
      <div className="metric-value">{value === undefined ? <span className="metric-dash">—</span> : value.toLocaleString()}</div>
      <div className="metric-foot"><span>{hint}</span>{href && <ArrowRight size={14} />}</div>
    </>
  );
  return href ? <Link className="metric-card panel" to={href}>{body}</Link> : <Card className="metric-card">{body}</Card>;
}

function ServiceRow({ title, detail, state, icon: Icon }: { title: string; detail: string; state: "ok" | "waiting" | "error"; icon: typeof Server }) {
  const statusIcon = state === "ok" ? <CheckCircle2 size={16} /> : state === "error" ? <XCircle size={16} /> : <Clock3 size={16} />;
  return (
    <div className="service-row">
      <span className="service-icon"><Icon size={16} /></span>
      <div className="service-copy"><strong>{title}</strong><span>{detail}</span></div>
      <span className={`service-state state-${state}`}>{statusIcon}{state === "ok" ? "Operational" : state === "error" ? "Unavailable" : "Checking"}</span>
    </div>
  );
}

function MatchRow({ match }: { match: RecentMatch }) {
  return (
    <div className="match-row">
      <div className="match-thumb-wrap">
        <img src={api.imageContentURL(match.image_id)} alt="" loading="lazy" className="match-thumb" />
      </div>
      <div className="match-main">
        <div className="match-title-row"><strong>{match.post_title}</strong><StatusPill status={match.accepted ? "processed" : "failed"}>{match.accepted ? "Accepted" : "Rejected"}</StatusPill></div>
        <span className="match-subtitle">{match.subject || "Image analysis pending"}{match.category ? ` · ${match.category}` : ""}</span>
        {!match.accepted && match.rejection_reason && <span className="match-reason">{match.rejection_reason}</span>}
        {match.latest_review && <span className="match-review">Reviewed {match.latest_review.decision}{match.review_count > 1 ? ` · ${match.review_count} actions` : ""}</span>}
      </div>
      <div className="match-score"><strong>{match.similarity.toFixed(3)}</strong><span>cosine</span></div>
      <time className="match-time" dateTime={match.created_at}>{formatDate(match.created_at)}</time>
    </div>
  );
}

export function OverviewPage() {
  const queryClient = useQueryClient();
  const imageQuery = useQuery({ queryKey: ["images", "overview-counts"], queryFn: () => api.listImages({ limit: 1 }) });
  const matchesQuery = useQuery({ queryKey: queryKeys.recentMatchesPage(6), queryFn: () => api.listMatches(6, 0) });
  const healthQuery = useQuery({ queryKey: ["health"], queryFn: api.health });
  const readyQuery = useQuery({ queryKey: ["readiness"], queryFn: api.readiness });
  const counts = imageQuery.data?.status_counts;
  const queued = counts ? counts.queued + counts.processing : undefined;
  const attention = counts ? counts.needs_review + counts.failed : undefined;

  const refresh = () => queryClient.invalidateQueries();

  return (
    <div className="page-stack">
      <section className="page-heading overview-heading">
        <div><SectionEyebrow>COMMAND CENTER <span /> LIVE WORKSPACE</SectionEyebrow><h1>Overview</h1><p>A live view of your image library and matching activity.</p></div>
        <Button onClick={refresh} aria-label="Refresh overview"><RefreshCw size={15} className={imageQuery.isFetching || matchesQuery.isFetching ? "spin-once" : ""} />Refresh data</Button>
      </section>

      {(imageQuery.isError || matchesQuery.isError) && (
        <div className="inline-alert" role="alert"><ShieldAlert size={17} /><span>{imageQuery.isError ? "Image totals are unavailable." : "Recent matching decisions are unavailable."} Check the local API connection and refresh.</span></div>
      )}

      <section className="metrics-grid" aria-label="Image processing totals">
        <MetricCard label="Total images" value={imageQuery.isLoading ? undefined : counts ? Object.values(counts).reduce((sum, value) => sum + value, 0) : undefined} hint="In this development tenant" icon={FileImage} href="/images" />
        <MetricCard label="Processed" value={imageQuery.isLoading ? undefined : counts?.processed} hint="Ready for matching" icon={CheckCircle2} tone="good" href="/images?status=processed" />
        <MetricCard label="In queue" value={imageQuery.isLoading ? undefined : queued} hint="Queued or processing" icon={Clock3} tone="blue" href="/images?status=queued" />
        <MetricCard label="Needs attention" value={imageQuery.isLoading ? undefined : attention} hint="Review or failed" icon={ShieldAlert} tone="amber" href="/images?status=needs_review" />
      </section>

      <section className="overview-columns">
        <Card className="recent-panel">
          <div className="panel-header">
            <div><SectionEyebrow>ACTIVITY</SectionEyebrow><CardHeading>Recent matching decisions</CardHeading></div>
            <span className="subtle-count">{matchesQuery.data ? `${matchesQuery.data.total} saved` : "Latest records"}</span>
          </div>
          {matchesQuery.isLoading ? <div className="match-skeletons"><Skeleton /><Skeleton /><Skeleton /></div> : matchesQuery.data?.items.length ? (
            <div className="match-list">{matchesQuery.data.items.map((match) => <MatchRow key={match.suggestion_id} match={match} />)}</div>
          ) : matchesQuery.isError ? (
            <div className="empty-state compact"><ShieldAlert size={22} /><strong>Could not load decision history</strong><span>Refresh after checking the API and database connection.</span></div>
          ) : (
            <div className="empty-state compact"><Sparkles size={22} /><strong>No matching decisions yet</strong><span>Persisted matches will appear here after matching is used.</span></div>
          )}
        </Card>

        <Card className="status-panel">
          <div className="panel-header"><div><SectionEyebrow>ENVIRONMENT</SectionEyebrow><CardHeading>System status</CardHeading></div><span className="local-lock"><span /> LOCAL</span></div>
          <div className="service-list">
            <ServiceRow title="Go API" detail="Liveness endpoint" state={healthQuery.data ? "ok" : healthQuery.isError ? "error" : "waiting"} icon={Server} />
            <ServiceRow title="PostgreSQL" detail="Readiness check" state={readyQuery.data?.status === "ready" ? "ok" : readyQuery.isError || readyQuery.data?.status === "not_ready" ? "error" : "waiting"} icon={Database} />
          </div>
          <div className="status-footnote"><ShieldAlert size={14} /><span>Development tenant only. Do not expose this API to untrusted networks.</span></div>
          <Link to="/images" className="panel-link">Browse image library <ArrowRight size={15} /></Link>
        </Card>
      </section>

      <section className="getting-started panel">
        <div className="getting-icon"><ImagePlus size={18} /></div>
        <div className="getting-copy"><strong>Your image workspace</strong><span>Upload a JPEG or PNG to start processing. Analysis is completed by the configured background worker.</span></div>
        <Link className="button button-primary button-sm" to="/images">Open image library <ArrowRight size={14} /></Link>
      </section>
    </div>
  );
}
