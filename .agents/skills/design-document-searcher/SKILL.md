---
name: design-document-searcher
description: Use when locating relevant engineering design documents, architecture design docs, feature design docs, or docs under docs/designs for a user request, code path, tag, feature, component, PR, issue, or implementation question.
---

# Design Document Searcher

## Overview

Find the design documents an engineer or AI agent should read before changing code or answering design-sensitive questions. Treat `index.yaml` as the routing table, then verify the selected documents by opening the actual files.

## Workflow

1. Identify the search target from the user request, explicit paths, changed files, feature names, tags, code symbols, issue text, or PR context. If the target is still ambiguous, ask one focused question.
2. Identify the design document location. Use the user's requested path or index file when provided; otherwise default to `docs/designs/index.yaml`.
3. If the target `index.yaml` exists, read it first. Do not start by scanning every design document unless the user asked for a full audit or the index is missing/stale.
4. Parse index entries and rank likely matches using the signals below.
5. Open the top candidate design documents to confirm relevance against frontmatter and body content. The index is a shortcut, not final proof.
6. Return a concise ranked list with the document path, title, status, and why each document is relevant. Include uncertain matches under a separate `Possible matches` group.
7. If no index exists, fall back to frontmatter discovery and clearly say that fallback was used.

## Index Rules

Default index path:

```text
docs/designs/index.yaml
```

Honor explicit user locations, including:

- A specific `index.yaml` path.
- A design document directory.
- A single design document path.

Expected index shape:

```yaml
version: 1
documents:
  - doc_id: <Copy from design document>
    path: <Design document path, such as: docs/designs/todo.md>
    title: <Copy from design document>
    status: <Copy from design document>
    code_paths:
      - <Copy from design document>
    tags:
      - <Copy from design document>
    summary: >
      <design document summary content>
```

Use `path` values as repository-relative paths. If an index entry points to a missing file, report that as an index problem instead of silently dropping the entry.

## Ranking Signals

Prefer strong, specific evidence over broad keyword overlap:

| Signal | Weight | How to use it |
| --- | --- | --- |
| Explicit document path or `doc_id` | Highest | Select that document and verify it exists. |
| `code_paths` matching changed or requested files | High | Match exact paths first, then glob-like prefixes or nearby directories. |
| User-provided feature/component terms in `title` or `doc_id` | High | Prefer exact phrase and normalized hyphen/space variants. |
| Tags matching domain, feature, component, or platform | Medium | Use tags to break ties and discover related docs. |
| Summary mentions the requested behavior or boundary | Medium | Good for conceptual questions without code paths. |
| Status | Medium | Prefer `accepted` and `implemented` for current behavior; include `draft` or `deprecated` only with context. |
| Body content | Confirming | Use after ranking to verify the document really answers the request. |

Do not claim a document is relevant from one weak keyword alone. Label it as possible if the evidence is thin.

## Frontmatter Fallback

Use this only when the target index is missing, unreadable, or explicitly bypassed by the user.

1. Determine the scan directory. Use the user's specified folder; otherwise use `docs/designs`.
2. Find Markdown design documents under that directory.
3. Read YAML frontmatter from each candidate and extract `doc_id`, `title`, `status`, `code_paths`, `tags`, and `summary`.
4. Rank the candidates with the same signals used for index entries.
5. Open matching documents and verify relevance against the body before reporting results.

Useful discovery commands:

```bash
test -f docs/designs/index.yaml
rg --files docs/designs -g '*.md'
rg '^---$|^doc_id:|^title:|^status:|^code_paths:|^tags:|^summary:' docs/designs -g '*.md'
```

If the fallback discovers useful documents, mention that creating or updating `index.yaml` with `$design-document-index-creator` would make future searches more precise.

## Output

For relevant matches, prefer this shape:

```markdown
**Relevant design documents**
- `docs/designs/example.md` (`design.example`, accepted): matches `src/example/**` in `code_paths`; summary covers the requested behavior.

**Possible matches**
- `docs/designs/adjacent.md` (`design.adjacent`, draft): shares tag `example`, but no direct code path match.

**Search basis**
- Used `docs/designs/index.yaml`; verified candidates by opening the documents.
```

If nothing matches, say what was searched and which signals were absent. Do not invent related documents.

## Common Mistakes

- Scanning all Markdown files before checking the default index.
- Returning index entries without opening the actual documents.
- Treating deprecated or draft documents as current truth without warning.
- Ignoring user-specified locations in favor of `docs/designs`.
- Equating broad word overlap with relevance.
- Hiding index problems such as missing files, duplicate `doc_id` values, or malformed entries.
