---
name: design-document-index-creator
description: Use when creating or updating index.yaml for engineering design documents, especially after writing, modifying, moving, or batch-normalizing Markdown design docs with YAML frontmatter under docs/designs or another user-specified design-doc directory.
---

# Design Document Index Creator

## Overview

Create and maintain `index.yaml` for Markdown design documents by reading each document's YAML frontmatter. Use the bundled script for parsing and merging instead of editing index entries by hand.

Read `references/index-template.yaml` when the exact index shape is needed.

## Workflow

1. Identify the design document location. Use the user's requested location when provided; otherwise default to `docs/designs`.
2. Make sure each design document has frontmatter fields: `doc_id`, `title`, `status`, `code_paths`, `tags`, and `summary`.
3. Resolve `SKILL_DIR` to this skill's installed directory, meaning the directory containing this `SKILL.md`.
4. From the target repository root, run `node "$SKILL_DIR/scripts/update_design_index.mjs"`.
5. Review the resulting `index.yaml` diff. It should contain one entry per `doc_id`, stable paths relative to the repository root, and sorted documents.
6. If the script reports missing frontmatter, update the design document first, then rerun the script.

## Commands

Do not assume this skill is installed inside the target repository. Set `SKILL_DIR` to the absolute path of the installed skill directory, then run commands from the target repository root:

```bash
SKILL_DIR=/absolute/path/to/design-document-index-creator
```

Update the default design-doc index by scanning `docs/designs`:

```bash
node "$SKILL_DIR/scripts/update_design_index.mjs"
```

Update the index for one edited design document:

```bash
node "$SKILL_DIR/scripts/update_design_index.mjs" docs/designs/todo.md
```

Update the index for a user-specified design-doc folder:

```bash
node "$SKILL_DIR/scripts/update_design_index.mjs" --scan path/to/designs
```

Write discovered documents to an explicit index path:

```bash
node "$SKILL_DIR/scripts/update_design_index.mjs" --scan docs/designs --index-path docs/designs/index.yaml
```

## Index Rules

- Default index path: `docs/designs/index.yaml`.
- For a specified document path, write `index.yaml` in that document's directory unless `--index-path` is provided.
- For `--scan <dir>`, write `<dir>/index.yaml` unless `--index-path` is provided.
- If `index.yaml` does not exist, create it.
- If an entry with the same `doc_id` exists, replace that entry with current frontmatter values.
- Preserve unrelated existing entries when updating a single document.
- Sort entries by `doc_id` for stable diffs.
- Store `path` relative to the repository root, using POSIX-style slashes.

## Script Notes

The script intentionally uses only Node.js built-in modules. It supports the YAML subset used by `design-document-creator`: top-level scalar fields, list fields, and folded block scalars such as `summary: >`.

Do not use the script for arbitrary YAML documents. If a design document uses unsupported frontmatter syntax, normalize the frontmatter to the design-document template before indexing.

## Verification

After changing this skill's script, run:

```bash
SKILL_DIR=/absolute/path/to/design-document-index-creator
node --test "$SKILL_DIR/tests/test_update_design_index.mjs" "$SKILL_DIR/tests/test_skill_docs.mjs"
```

When editing the skill package itself, also run the local skill-creator `quick_validate.py` against the same `SKILL_DIR`.
