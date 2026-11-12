# Development roadmap

| Capability | Source status | Exit evidence still needed |
|---|---|---|
| Phase 0 — Discovery and design | Complete | Design artifacts are in the repository. |
| Phase 1 — API/database foundation | Complete | Previously verified health/readiness, PostgreSQL, pgvector, and seed. |
| Image ingestion and background jobs | Implemented and tested end to end | 40 curated JPEGs reached `processed`; metadata and image vectors persisted in the isolated fresh database. |
| Gemini image analysis and embeddings | Implemented and tested end to end | Structured classification succeeded; 40 image and 11 article embeddings persisted at 768 dimensions. |
| Matching and mismatch guard | Implemented and tested end to end | Fox accepted, gray wolf rejected, unrelated geology article abstained. |
| Human review | Implemented and tested end to end | Approval and rejection API calls returned 201; audit rows preserved original AI decisions. |
| Evaluation and hardening | Implemented and tested on curated set | 40 licensed images, four categories, 11 source-derived articles; metrics are in `EVIDENCE.md`. |
| Production tenant isolation | Not implemented | API-key authentication, tenant authorization, and production route policy. |

Evaluation labels have one manual visual review and no independent adjudication. The measured score is demonstration evidence, not a broad quality claim. Production tenant authentication remains planned and is not implemented.
