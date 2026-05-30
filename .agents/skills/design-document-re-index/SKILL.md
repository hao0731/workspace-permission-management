---
name: design-document-re-index
description: Use when checking whether engineering design document index.yaml files match Markdown design document frontmatter, especially after design docs are created, edited, moved, renamed, normalized, or suspected to have stale index entries.
---

# Design Document Re-Index

## Overview

Check whether a design-document `index.yaml` is in sync with Markdown design document frontmatter. Treat the output from `$design-document-index-creator` as canonical; if the generated canonical index differs from the current index, rebuild the current index with `$design-document-index-creator`.

## Workflow

1. Identify the design document location. Use the user's requested directory or index path when provided; otherwise default to `docs/designs` and `docs/designs/index.yaml`.
2. Resolve `CREATOR_SKILL_DIR` to the installed `design-document-index-creator` skill directory. If command options or index rules are unclear, read that skill before proceeding.
3. From the target repository root, generate a temporary canonical index with the creator script and `--index-path` pointing outside the repository.
4. Compare the temporary canonical index against the current `index.yaml`.
5. If the current index is missing or differs, run the creator script again with the real index path to rebuild it.
6. Re-run the temporary generation and comparison after rebuilding. Report whether the index is now synchronized and summarize any resulting diff.
7. If the creator script reports missing or unsupported frontmatter, fix or request fixes to the design document frontmatter before rebuilding the index.

## Commands

Do not edit `index.yaml` by hand. Set `CREATOR_SKILL_DIR` to the absolute path of the installed `design-document-index-creator` skill, then run commands from the target repository root.

Default check for `docs/designs`:

```bash
CREATOR_SKILL_DIR=/absolute/path/to/design-document-index-creator
SCAN_DIR=docs/designs
INDEX_PATH=docs/designs/index.yaml
TMP_DIR="$(mktemp -d)"
EXPECTED_INDEX="$TMP_DIR/index.yaml"
node "$CREATOR_SKILL_DIR/scripts/update_design_index.mjs" --scan "$SCAN_DIR" --index-path "$EXPECTED_INDEX"
test -f "$INDEX_PATH" && diff -u "$INDEX_PATH" "$EXPECTED_INDEX"
```

If `index.yaml` is missing or the diff reports differences, rebuild the real index:

```bash
node "$CREATOR_SKILL_DIR/scripts/update_design_index.mjs" --scan "$SCAN_DIR" --index-path "$INDEX_PATH"
```

For a user-specified design-doc directory, change `SCAN_DIR` and `INDEX_PATH`, then run the same check. Treat diff exit code `1` as a mismatch; treat creator-script errors as frontmatter or path issues to resolve before rebuilding.

```bash
SCAN_DIR=path/to/designs
INDEX_PATH="$SCAN_DIR/index.yaml"
TMP_DIR="$(mktemp -d)"
EXPECTED_INDEX="$TMP_DIR/index.yaml"
node "$CREATOR_SKILL_DIR/scripts/update_design_index.mjs" --scan "$SCAN_DIR" --index-path "$EXPECTED_INDEX"
test -f "$INDEX_PATH" && diff -u "$INDEX_PATH" "$EXPECTED_INDEX"
```

When the user provides an explicit index path, infer the scan directory from the index path's directory unless the user also provides a different scan directory.

## Consistency Rules

The canonical temporary index defines correctness. Any textual difference means the current index is stale, including changes to:

- document presence or ordering
- `doc_id`
- `path`
- `title`
- `status`
- `code_paths`
- `tags`
- `summary`
- creator-managed YAML formatting

If `index.yaml` does not exist but design documents do exist, treat the index as stale and create it with `$design-document-index-creator`.

## Reporting

When synchronized, say which scan directory and index path were checked. When rebuilt, mention that `$design-document-index-creator` regenerated the index and summarize the changed documents or fields from the diff. When blocked, report the exact frontmatter or parsing issue from the creator script.

## Common Mistakes

- Manually patching `index.yaml` instead of using `$design-document-index-creator`.
- Comparing only selected fields with `rg` and missing formatting, ordering, path, or summary drift.
- Declaring the index current when the creator script failed due invalid frontmatter.
- Ignoring user-specified scan directories or explicit index paths.
