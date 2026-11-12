# Evaluation dataset and metrics

The evaluation CLI operates on persisted database records. It requires at least 40 unique, licensed, processed, non-uncertain images across four or more categories, plus at least ten unique posts with `positive`, `mismatch_guard`, and `no_suitable` scenarios. Each image manifest row includes an HTTP(S) source URL and license. The category labels come from the reviewed manifest, not the model's broader vision category such as `mammal` or `bird`.

## Curated demonstration set

[`evaluation-data/curated/`](../evaluation-data/curated/) contains 40 attributed images: ten red foxes, ten gray wolves, ten cheetahs, and ten bald eagles. `article_sources.csv` records ten source-derived positive article posts, one unrelated geology control, a source page, a category label, and one visually checked positive example image per relevant post. `ATTRIBUTION.md` provides image credits and license links.

The labels and visual checks have one reviewer and no independent human adjudication. The set is sufficient to exercise the current demonstration pipeline, but its scores are not an independently validated quality benchmark.

Stable source data is stored in `sources.csv`, `article_sources.csv`, and `articles.json`. The 40 JPEGs total 7,656,543 bytes and are excluded from Git; `sources.csv` records each SHA-256 alongside Commons source and license metadata. On a fresh checkout, run `python3 scripts/download-curated-dataset.py` to download missing images and verify checksums. Database UUIDs are generated per database and written to the ignored `evaluation-data/curated/runtime/` directory by `scripts/bootstrap-evaluation.py`; they are not stable dataset identifiers.

## CSV formats

`images.csv`:

```csv
image_id,category,source_url,license
```

Each `image_id` must already identify a processed, non-uncertain image in the development tenant. Retain creator/credit and license URL in source metadata.

`articles.csv`:

```csv
post_id,expected_image_ids,negative_image_ids,scenario
```

Use `|` between multiple IDs. Positive and mismatch-guard cases need expected image IDs. A mismatch-guard row also needs one or more known negative candidates. A no-suitable row has an empty expected field; any accepted recommendation is incorrect.

## Rebuild the runtime manifests and evaluate

```sh
python3 scripts/download-curated-dataset.py
docker compose --profile ai up --build -d
python3 scripts/bootstrap-evaluation.py
```

The bootstrap seeds or identifies the development tenant, uploads or reuses the 40 attributed JPEGs, waits for Gemini processing and embeddings, creates or reuses all 11 article records, writes fresh UUID manifests, then invokes the evaluation CLI. It requires Docker Hub access for the Go build image and Google Gemini API access for real provider processing. It does not remove the configured PostgreSQL volume.

To rerun the CLI after bootstrap:

```sh
docker compose exec -T api /evaluate \
  -images /evaluation-data/curated/runtime/images.csv \
  -articles /evaluation-data/curated/runtime/articles.csv
```

The CLI verifies the manifest and database rows, calls the actual matcher for each post, and prints a JSON report:

- `image_count`, `category_count`, and `article_count` describe the manifest.
- `top1_precision` is relevant positive/mismatch-guard cases whose first accepted suggestion is a labeled image, divided by relevant cases. A missing accepted suggestion is incorrect. The no-suitable control is excluded from this denominator.
- `recommendation_coverage` is cases with one or more accepted suggestions divided by all cases, including no-suitable cases.
- `accepted_recommendation_precision` is accepted suggestions whose image ID is in that case's expected set divided by all accepted suggestions.
- `incorrect_accepted_matches` counts accepted suggestions outside the expected set.
- `abstentions` counts cases with no accepted suggestions.

## Observed output — 2026-11-12

```json
{
  "image_count": 40,
  "category_count": 4,
  "article_count": 11,
  "top1_precision": 1,
  "recommendation_coverage": 0.9090909090909091,
  "accepted_recommendation_precision": 1,
  "incorrect_accepted_matches": 0,
  "abstentions": 1
}
```

This is 10/10 correct top-1 results among the ten relevant posts, 10/11 coverage, no incorrect accepted matches, and one no-suitable abstention. The CLI reported accepted-recommendation precision `1.0`; it does not emit the accepted recommendation count. The values describe this one-reviewer demonstration set.
