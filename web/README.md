# Image Relevance Studio

The React frontend for the `image-relevance` Go API. Implemented screens include the responsive application shell and live overview, the image library with upload and processing polling, Matching Studio for article creation and API-backed matching, Decision Inspector for candidate evidence and backend decisions, and Review Centre for review actions and persisted audit history. Matching results and image metadata come from the API; sample article templates are clearly presented as editable starting points.

## Local development

Requirements: Node.js 22+ and npm 10+.

From the repository root, start PostgreSQL and the Go API with the project workflow. The host-run API defaults to loopback at `127.0.0.1:8080`. In a second terminal:

```sh
cd web
npm ci
npm run dev
```

Open <http://127.0.0.1:5173>. Vite proxies `/api/*` to `http://127.0.0.1:8080` and removes the `/api` prefix. Run the Go API separately. The frontend binds only to loopback and has no API key or database configuration; do not deploy this development-only UI publicly.

If port `5173` is already in use, run `npm run dev -- --port 5174` and open <http://127.0.0.1:5174> instead.

The backend's business routes use the seeded development tenant and do not authenticate a caller. The environment indicator is always shown. PostgreSQL must be ready to list or upload images; Gemini and the worker are needed to move newly uploaded items through analysis.

## Verification

```sh
npm run typecheck
npm test
npm run build
```

The component tests mock the network boundary and cover navigation, image upload and processing polling, article creation and matching results, mismatch rejection, no-confident-match rendering, decision inspection, and review submission/history behavior. They do not claim a live database or Gemini run; those results are tracked separately in the repository evidence.
