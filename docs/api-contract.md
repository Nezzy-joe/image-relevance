# API contract

All routes return JSON. Standard errors use `{ "error": { "code": "...", "message": "...", "request_id": "..." } }`. Operational endpoints return their own status/check response bodies. Internal SQL/provider details are not returned to clients.

Business routes are registered only when `APP_ENV=development`. Tenant API-key authentication is not implemented, so do not expose this API to untrusted networks or use the development tenant as production isolation.

## Operational endpoints

### `GET /health`

Process liveness only; it does not query PostgreSQL. Returns HTTP `200` while the process serves requests:

```json
{"status":"ok"}
```

### `GET /ready`

Pings PostgreSQL with the configured timeout. When the database responds, it returns HTTP `200`:

```json
{"status":"ready","checks":{"database":"ok"}}
```

When PostgreSQL is unavailable or unconfigured, it returns HTTP `503`:

```json
{"status":"not_ready","checks":{"database":"unavailable"}}
```

## Development image routes

### `GET /v1/images`

Returns a tenant-scoped page using `limit` (default 24, maximum 100) and `offset`. Optional `q` searches the checksum, validated subject, category, and caption. Optional `status`, `category`, and `subject` filters apply to persisted records. The response includes `{items,total,limit,offset,status_counts}`; `status_counts` covers the full development tenant, not only the current page. Analysis metadata is included only when it passes the domain schema validation. Invalid filters return HTTP `400`.

### `POST /v1/images`

Accepts `multipart/form-data` with a `file` field. Only valid JPEG and PNG data is accepted; maximum size defaults to 10 MiB. The server calculates SHA-256 and deduplicates within the development tenant. A new image returns HTTP `202` and creates a `queued` image plus an `analyze_image` job. Existing content returns HTTP `200` with the existing image state. Requires a seeded tenant; actual processing requires the optional Gemini worker profile and `GEMINI_API_KEY`.

### `GET /v1/images/{imageID}`

Returns the image checksum, MIME type, byte size, lifecycle state, and validated metadata when processing has completed.

Image state values are `queued`, `processing`, `processed`, `needs_review`, and `failed`. Low-confidence analysis is stored with `uncertain=true` and state `needs_review`; it is not eligible for automatic recommendation.

### `GET /v1/images/{imageID}/content`

Streams the original JPEG or PNG bytes for an image belonging to the development tenant. The content lookup is tenant-scoped and opens only generated SHA-256 storage keys through a rooted filesystem API. Responses include `X-Content-Type-Options: nosniff` and `Cache-Control: private, no-store`; missing rows or files return the standard JSON `404` error envelope.

## Development post, matching, and review routes

Article creation and matching require the development tenant and a configured Gemini client in the API process. Match-history reads and review actions require the seeded tenant and PostgreSQL, but do not call Gemini.

### `GET /v1/matches`

Returns paginated recent match suggestions persisted by matching requests. The response joins only posts and images in the same development tenant and includes article/image metadata, candidate rank, the original accepted decision and rejection reason, the latest append-only review action, and review count. `limit` defaults to 20 (maximum 100); `offset` defaults to 0. This route does not replace or mutate AI decisions.

### `POST /v1/posts`

Accepts JSON:

```json
{"title":"Red fox behaviour","body":"Article text..."}
```

Returns HTTP `201` with post ID, resolved subject, subject confidence, and creation time. Subject resolution and article embedding are provider calls; usage records capture available token/latency fields.

### `POST /v1/posts/{postID}/matches`

Retrieves vector-nearest processed image candidates, then applies normalized phrase-aware subject compatibility, article subject confidence, image uncertainty, and similarity floors. A more specific article subject such as `red fox hunting behavior` can match the image subject `red fox`; a different subject such as `gray wolf` is rejected. Returns HTTP `200`. Compatible candidates appear in `suggestions`; rejected nearest candidates appear in `rejected` with a human-readable reason. A no-match response has this shape:

```json
{
  "status":"no_confident_match",
  "suggestions":[],
  "reasons":["No candidate satisfied the subject and similarity requirements."],
  "rejected":[]
}
```

Match decisions are inserted as immutable suggestion records; later runs create new records rather than overwriting prior decisions.

### `POST /v1/matches/{suggestionID}/review`

Accepts JSON `{"decision":"approved"}` or `{"decision":"rejected","comment":"..."}`. Returns HTTP `201`. Every review creates an append-only `review_actions` row and leaves the original suggestion decision intact. Review actions are available in development even when the Gemini provider is not configured.

### `GET /v1/matches/{suggestionID}/reviews`

Returns the tenant-scoped append-only review audit history for one persisted suggestion, newest first. Accepts `limit` (default 20, maximum 100) and `offset` (default 0), and returns `{items,total,limit,offset}`. Each item includes the review action ID, decision, optional comment, and creation timestamp. A suggestion outside the development tenant returns `404`.

## JSON errors

Malformed request bodies return HTTP `400`. Missing posts or suggestions return `404`. Operational database unavailability returns `503`. Other internal failures use a stable error code and request ID without exposing SQL, credentials, or provider diagnostics.
