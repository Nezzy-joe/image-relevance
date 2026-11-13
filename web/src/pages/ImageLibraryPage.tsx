import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertCircle,
  Check,
  ChevronLeft,
  ChevronRight,
  CloudUpload,
  FileImage,
  ImageOff,
  LoaderCircle,
  Search,
  X,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Button, Card, CardHeading, SectionEyebrow, Skeleton, StatusPill } from "../components/ui/primitives";
import { ApiError, api } from "../lib/api";
import { queryKeys } from "../lib/query-keys";
import type { ImageRecord, ImageStatus } from "../types/api";

const PAGE_SIZE = 24;
const MAX_IMAGE_BYTES = 10 * 1024 * 1024;
const finalStates: ImageStatus[] = ["processed", "needs_review", "failed"];
const statuses: { value: ImageStatus | ""; label: string }[] = [
  { value: "", label: "All states" },
  { value: "queued", label: "Queued" },
  { value: "processing", label: "Processing" },
  { value: "processed", label: "Processed" },
  { value: "needs_review", label: "Needs review" },
  { value: "failed", label: "Failed" },
];

type Notice = { kind: "success" | "error" | "info"; text: string };

function bytesLabel(bytes: number) {
  return bytes >= 1024 * 1024 ? `${(bytes / (1024 * 1024)).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`;
}

function readableError(error: unknown) {
  if (error instanceof ApiError) return error.message;
  return "The request could not be completed. Try again.";
}

function ImagePreview({ imageID, alt, className = "" }: { imageID: string; alt: string; className?: string }) {
  const [failed, setFailed] = useState(false);
  return (
    <div className={`image-preview ${className}`}>
      {failed ? <div className="image-fallback"><ImageOff size={22} /><span>Preview unavailable</span></div> : <img src={api.imageContentURL(imageID)} alt={alt} loading="lazy" onError={() => setFailed(true)} />}
    </div>
  );
}

function ProcessingTracker({ imageID, onComplete }: { imageID: string; onComplete: (record: ImageRecord) => void }) {
  const query = useQuery({
    queryKey: queryKeys.image(imageID),
    queryFn: () => api.getImage(imageID),
    refetchInterval: (current) => {
      if (current.state.status === "error") return false;
      const status = current.state.data?.status;
      return status && finalStates.includes(status) ? false : 1500;
    },
  });
  const completed = useRef(false);
  useEffect(() => {
    if (query.data && finalStates.includes(query.data.status) && !completed.current) {
      completed.current = true;
      onComplete(query.data);
    }
  }, [query.data, onComplete]);
  if (query.isError) return <div className="tracker-line tracker-error"><AlertCircle size={15} />Could not check processing status. The upload was stored.<Button variant="ghost" size="sm" onClick={() => { void query.refetch(); }}>Retry status check</Button></div>;
  const status = query.data?.status ?? "queued";
  return <div className={`tracker-line tracker-${status}`}><LoaderCircle size={15} className={finalStates.includes(status) ? "" : "spin"} />{query.data ? `Image ${status.replaceAll("_", " ")}` : "Upload accepted · waiting for processing state…"}</div>;
}

function ImageCard({ image, selected, onSelect }: { image: ImageRecord; selected: boolean; onSelect: () => void }) {
  const metadata = image.metadata;
  return (
    <button type="button" className={`image-card${selected ? " selected" : ""}`} onClick={onSelect} aria-pressed={selected}>
      <ImagePreview imageID={image.id} alt={metadata?.caption ?? "Uploaded image"} className="image-card-preview" />
      <div className="image-card-body">
        <div className="image-card-title-row"><strong>{metadata?.subject ?? "Image awaiting analysis"}</strong><StatusPill status={image.status} /></div>
        <span className="image-card-detail">{metadata?.category ?? image.mime_type} <i /> {bytesLabel(image.size_bytes)}</span>
        <div className="image-card-footer"><span>{new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric" }).format(new Date(image.created_at))}</span>{metadata && <span className="confidence-tag">{Math.round(metadata.confidence * 100)}% confidence</span>}</div>
      </div>
    </button>
  );
}

function MetadataPanel({ image, onClose }: { image: ImageRecord; onClose: () => void }) {
  return (
    <Card className="metadata-panel">
      <div className="metadata-head"><div><SectionEyebrow>IMAGE DETAILS</SectionEyebrow><CardHeading>Inspection</CardHeading></div><Button variant="ghost" size="icon" aria-label="Close image details" onClick={onClose}><X size={17} /></Button></div>
      <ImagePreview imageID={image.id} alt={image.metadata?.caption ?? "Selected image"} className="metadata-preview" />
      <div className="metadata-status-row"><span>Processing state</span><StatusPill status={image.status} /></div>
      {image.metadata ? (
        <div className="metadata-fields">
          <div><span>Detected subject</span><strong>{image.metadata.subject}</strong></div>
          <div><span>Category</span><strong>{image.metadata.category}</strong></div>
          <div><span>Confidence</span><strong>{Math.round(image.metadata.confidence * 100)}%</strong><div className="confidence-meter"><span style={{ width: `${image.metadata.confidence * 100}%` }} /></div></div>
          <div><span>Caption</span><p>{image.metadata.caption}</p></div>
          <div><span>Attributes</span><div className="tag-row">{image.metadata.attributes.map((attribute) => <span className="metadata-tag" key={attribute}>{attribute}</span>)}</div></div>
        </div>
      ) : <div className="metadata-pending"><LoaderCircle size={16} className={image.status === "queued" || image.status === "processing" ? "spin" : ""} /><span>{image.status === "failed" ? "Analysis failed. Re-upload the image to retry." : image.status === "needs_review" ? "Analysis requires review; structured metadata is not available." : "Metadata will appear when image analysis finishes."}</span></div>}
      <div className="metadata-file"><span>File</span><strong>{image.mime_type} · {bytesLabel(image.size_bytes)}</strong></div>
      <div className="metadata-hash"><span>SHA-256</span><code>{image.sha256.slice(0, 18)}…</code></div>
    </Card>
  );
}

export function ImageLibraryPage() {
  const [searchParams] = useSearchParams();
  const [search, setSearch] = useState("");
  const [subject, setSubject] = useState("");
  const [category, setCategory] = useState("");
  const [status, setStatus] = useState<ImageStatus | "">(() => {
    const initial = searchParams.get("status") ?? "";
    return statuses.some((item) => item.value === initial) ? initial as ImageStatus | "" : "";
  });
  const [offset, setOffset] = useState(0);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [activeUploadIDs, setActiveUploadIDs] = useState<string[]>([]);
  const [notice, setNotice] = useState<Notice | null>(null);
  const [dragging, setDragging] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const queryClient = useQueryClient();

  const imageQuery = useQuery({
    queryKey: [...queryKeys.images, { limit: PAGE_SIZE, offset, q: search.trim(), status, category: category.trim(), subject: subject.trim() }],
    queryFn: () => api.listImages({ limit: PAGE_SIZE, offset, q: search.trim(), status, category: category.trim(), subject: subject.trim() }),
  });
  const uploadMutation = useMutation({
    mutationFn: api.uploadImage,
    onSuccess: (result) => {
      setActiveUploadIDs((current) => current.includes(result.image_id) ? current : [...current, result.image_id]);
      setSelectedID(result.image_id);
      setOffset(0);
      setNotice({ kind: "info", text: result.existing ? "This image already exists in the development tenant. Checking its current state…" : "Upload accepted. The worker will analyze this image asynchronously." });
      void queryClient.invalidateQueries({ queryKey: queryKeys.images });
    },
    onError: (error) => setNotice({ kind: "error", text: readableError(error) }),
  });

  const selectedFromPage = imageQuery.data?.items.find((image) => image.id === selectedID);
  const selectedQuery = useQuery({
    queryKey: queryKeys.image(selectedID ?? ""),
    queryFn: () => api.getImage(selectedID!),
    enabled: Boolean(selectedID) && !activeUploadIDs.includes(selectedID ?? ""),
  });
  const selectedImage = selectedQuery.data ?? selectedFromPage;
  const images = imageQuery.data?.items ?? [];
  const pages = Math.max(1, Math.ceil((imageQuery.data?.total ?? 0) / PAGE_SIZE));
  const pageNumber = Math.floor(offset / PAGE_SIZE) + 1;

  const filters = useMemo(() => ({ search, subject, category, status }), [search, subject, category, status]);
  const hasActiveFilters = Object.values(filters).some((value) => value.trim().length > 0);
  useEffect(() => setOffset(0), [filters]);

  function chooseFile(file?: File) {
    if (!file) return;
    if (file.type !== "image/jpeg" && file.type !== "image/png") {
      setNotice({ kind: "error", text: "Choose a valid JPEG or PNG image." });
      return;
    }
    if (file.size > MAX_IMAGE_BYTES) {
      setNotice({ kind: "error", text: "The image exceeds the 10 MiB upload limit." });
      return;
    }
    setNotice(null);
    uploadMutation.mutate(file);
    if (fileRef.current) fileRef.current.value = "";
  }

  const processingComplete = (record: ImageRecord) => {
    setNotice(record.status === "failed"
      ? { kind: "error", text: "Image processing failed. The stored image is available for inspection." }
      : { kind: "success", text: `Image processing finished: ${record.status.replaceAll("_", " ")}.` });
    setActiveUploadIDs((current) => current.filter((imageID) => imageID !== record.id));
    void queryClient.invalidateQueries({ queryKey: queryKeys.images });
    void queryClient.invalidateQueries({ queryKey: queryKeys.image(record.id) });
  };

  return (
    <div className="page-stack">
      <section className="page-heading library-heading">
        <div><SectionEyebrow>ASSET WORKSPACE <span /> {imageQuery.data ? `${imageQuery.data.total} IMAGES` : "IMAGE LIBRARY"}</SectionEyebrow><h1>Image Library</h1><p>Upload, process, and inspect images stored in your local development tenant.</p></div>
        <Button variant="primary" onClick={() => fileRef.current?.click()} disabled={uploadMutation.isPending}><CloudUpload size={16} />Upload image</Button>
      </section>

      <input ref={fileRef} type="file" accept="image/jpeg,image/png,.jpg,.jpeg,.png" className="visually-hidden" aria-label="Choose JPEG or PNG image" onChange={(event) => chooseFile(event.currentTarget.files?.[0])} />

      <button type="button" className={`upload-dropzone${dragging ? " drag-active" : ""}${uploadMutation.isPending ? " upload-pending" : ""}`} onClick={() => !uploadMutation.isPending && fileRef.current?.click()} onDragOver={(event) => { event.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={(event) => { event.preventDefault(); setDragging(false); chooseFile(event.dataTransfer.files[0]); }} disabled={uploadMutation.isPending}>
        <span className="dropzone-icon">{uploadMutation.isPending ? <LoaderCircle size={19} className="spin" /> : <CloudUpload size={20} />}</span>
        <span className="dropzone-copy"><strong>{uploadMutation.isPending ? "Uploading image…" : "Drop an image here, or browse files"}</strong><span>JPEG or PNG · up to 10 MiB · analysis runs in the background</span></span>
        <span className="dropzone-action">Choose file <ChevronRight size={14} /></span>
      </button>

      {notice && <div className={`notice notice-${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}><span>{notice.kind === "success" ? <Check size={15} /> : <AlertCircle size={15} />}{notice.text}</span><button aria-label="Dismiss notification" onClick={() => setNotice(null)}><X size={15} /></button></div>}
      {activeUploadIDs.map((imageID) => <ProcessingTracker key={imageID} imageID={imageID} onComplete={processingComplete} />)}

      <section className="library-toolbar panel" aria-label="Image filters">
        <label className="search-field"><Search size={16} /><span className="visually-hidden">Search images</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search subject, caption, or checksum" /></label>
        <label className="select-filter"><span className="visually-hidden">Filter by status</span><select value={status} onChange={(event) => setStatus(event.target.value as ImageStatus | "")}>{statuses.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>
        <label className="filter-input"><span className="visually-hidden">Filter by category</span><input value={category} onChange={(event) => setCategory(event.target.value)} placeholder="Category" /></label>
        <label className="filter-input"><span className="visually-hidden">Filter by subject</span><input value={subject} onChange={(event) => setSubject(event.target.value)} placeholder="Subject" /></label>
      </section>

      {imageQuery.isError && <div className="inline-alert" role="alert"><AlertCircle size={17} /><span>{readableError(imageQuery.error)} Image data is unavailable until the local API and database are ready.</span><Button variant="ghost" size="sm" onClick={() => void imageQuery.refetch()}>Retry</Button></div>}

      <div className={`library-layout${selectedImage ? " with-inspector" : ""}`}>
        <section className="library-results" aria-label="Image results" aria-busy={imageQuery.isLoading}>
          <div className="results-heading"><div><SectionEyebrow>LIBRARY</SectionEyebrow><h2>{status ? `${status.replaceAll("_", " ")} images` : "All images"}</h2></div><span className="results-count">{imageQuery.data ? `${imageQuery.data.total} results` : "Live records"}</span></div>
          {imageQuery.isLoading ? (
            <div className="image-grid">{Array.from({ length: 6 }, (_, index) => <div className="image-skeleton" key={index}><Skeleton className="skeleton-image" /><Skeleton className="skeleton-line" /><Skeleton className="skeleton-line short" /></div>)}</div>
          ) : images.length ? (
            <div className="image-grid">{images.map((image) => <ImageCard key={image.id} image={image} selected={image.id === selectedID} onSelect={() => setSelectedID((current) => current === image.id ? null : image.id)} />)}</div>
          ) : imageQuery.isError ? null : (
            <Card className="library-empty"><div className="empty-art"><FileImage size={23} /></div><h3>{hasActiveFilters ? "No images match these filters" : "No images in this workspace"}</h3><p>{hasActiveFilters ? "Try adjusting the search or clearing a filter." : "Upload a JPEG or PNG to start building your image library."}</p>{hasActiveFilters ? <Button variant="secondary" onClick={() => { setSearch(""); setCategory(""); setSubject(""); setStatus(""); }}>Clear filters</Button> : <Button variant="primary" onClick={() => fileRef.current?.click()}><CloudUpload size={15} />Upload first image</Button>}</Card>
          )}
          {imageQuery.data && imageQuery.data.total > PAGE_SIZE && <div className="pagination"><span>Page {pageNumber} of {pages}</span><div><Button variant="secondary" size="sm" aria-label="Previous page" disabled={offset === 0} onClick={() => setOffset((value) => Math.max(0, value - PAGE_SIZE))}><ChevronLeft size={15} />Previous</Button><Button variant="secondary" size="sm" aria-label="Next page" disabled={offset + PAGE_SIZE >= imageQuery.data.total} onClick={() => setOffset((value) => value + PAGE_SIZE)}>Next<ChevronRight size={15} /></Button></div></div>}
        </section>
        {selectedImage && <MetadataPanel image={selectedImage} onClose={() => setSelectedID(null)} />}
        {selectedID && !selectedImage && selectedQuery.isError && <Card className="metadata-panel metadata-error"><Button variant="ghost" size="icon" aria-label="Close image details" onClick={() => setSelectedID(null)}><X size={17} /></Button><AlertCircle size={21} /><strong>Image details unavailable</strong><span>{readableError(selectedQuery.error)}</span></Card>}
        {selectedID && !selectedImage && selectedQuery.isLoading && <Card className="metadata-panel"><Skeleton className="skeleton-image" /><Skeleton className="skeleton-line" /><Skeleton className="skeleton-line short" /></Card>}
      </div>
    </div>
  );
}
