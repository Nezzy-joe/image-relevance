# Curated evaluation image set

This dataset contains 40 visually reviewed JPEGs, with 10 images each for red fox, gray wolf, cheetah, and bald eagle. Ten files from the original source pool were excluded after visual inspection: three gray-wolf group results (African wolf, Ethiopian wolf, and a statue), six cheetah group results (painting, aircraft, bowls), and one African fish eagle. They remain in the unmodified raw source pool but are absent from this curated set. Ten licensed Commons photos were added as replacements.

`sources.csv` and `ATTRIBUTION.md` retain each image's Commons source, creator/credit, and license. `article_sources.csv` links ten positive evaluation posts and one unrelated control to their source pages and category labels. Each positive post has a visually checked example image; all same-species images are labeled relevant at the category level, and the other species categories are negative.

The 40 JPEGs total 7,656,543 bytes (about 7.3 MiB) and are excluded from Git. `sources.csv` includes a SHA-256 digest, Commons source page, author/credit, license, and license URL for every asset. Run `python3 scripts/download-curated-dataset.py` to fetch missing assets and verify their checksums. Existing local files are preserved.

The review and labels have one visual reviewer and no independent human adjudication. The scores are for this recorded demonstration run and are not independently adjudicated benchmark ground truth. `sources.csv`, `article_sources.csv`, and `articles.json` are the stable dataset inputs. `scripts/bootstrap-evaluation.py` generates database-specific `runtime/images.csv`, `runtime/articles.csv`, and `runtime/id-mapping.json`; the runtime directory is ignored by Git.
