---
name: design-document-creator
description: Use when creating, updating, reviewing, or normalizing engineering design documents, architecture design docs, feature design docs, ADR-adjacent design notes, or docs under docs/designs that need frontmatter, scope boundaries, proposed design, decisions, alternatives, invariants, error handling, testing strategy, and related documents.
---

# Design Document Creator

## Overview

Create design documents that are useful to both engineers and AI agents. Keep the document focused on system design, boundaries, trade-offs, and durable constraints instead of turning it into an implementation plan.

Before drafting, read `references/design-document-template.md` when the exact section structure, frontmatter, or placeholder wording is needed.

## Workflow

1. Gather the design context from the user request, existing docs, and relevant code. Prefer `rg --files`, `rg`, and nearby docs before making assumptions.
2. If core information is unclear, ask concise questions until the design can be written safely. Do not ask about details that can be inferred from the repository or the user's request.
3. Choose the output path. Use the user's requested folder when provided; otherwise use `docs/designs`.
4. Choose a stable file name, usually `design.<domain-or-feature>.md`, matching the `doc_id` frontmatter.
5. Draft YAML frontmatter first, then the design body.
6. Use the required sections below for every design document. Include conditional sections when relevant; when a kept section is not applicable, write `No changes.` or `Not applicable.`
7. Verify the draft before finishing: required frontmatter exists, required sections exist, no unresolved template placeholders remain, and the body describes design rather than step-by-step implementation.

## Required Frontmatter

Every design document must start with this YAML frontmatter shape:

```yaml
---
doc_id: design.<domain-or-feature>
doc_type: design
title: <Human readable title>
status: draft | accepted | implemented | deprecated

tags:
  - <domain>
  - <feature>
  - <important-keyword>

code_paths:
  - <glob-pattern>

related:
  designs: []
  adrs: []

last_updated_at: YYYY-MM-DD

summary: >
  One or two sentences describing when an engineer or AI Agent should read
  this document.
---
```

Set `last_updated_at` to the current date. Use `draft` unless the user or repository context clearly indicates another status.

## Required Sections

Every design document must include these sections:

- `Summary`
- `Scope`
- `Non-Scope`
- `Background / Current State`
- `Goals`
- `Non-Goals`
- `Proposed Design`
- `Design Decisions`
- `Alternatives Considered`
- `Invariants`
- `Error Handling`
- `Testing Strategy`
- `Related Documents`

The required sections must contain real design content. If related documents do not exist, say `None identified.` rather than inventing references.

## Conditional Sections

Include these sections when they are relevant to the design, or keep the heading with `No changes.` / `Not applicable.` when preserving the full canonical template:

- `API / Contract Changes`
- `Data Model / Persistence Changes`
- `Configuration / Environment Changes`
- `Security / Privacy Considerations`
- `Observability`
- `Rollout / Migration Plan`
- `Risks and Mitigations`
- `Open Questions`
- `Design Drift Notes`
- `Revision History`

Prefer preserving the canonical numbering from the template. It is acceptable for a short design to include optional sections with concise `Not applicable.` content.

## Writing Rules

- Write for future engineers and AI agents who will use the document to make implementation decisions.
- Describe architecture, ownership boundaries, contracts, invariants, trade-offs, and behavior.
- Do not write an implementation plan. Avoid ordered task lists such as "create service, then add controller, then write tests" except where describing runtime data flow or rollout sequencing.
- Define what each component must do and must not do.
- Capture alternatives that were considered and why they were not chosen.
- Record design decisions with rationale, consequences, and related ADR references when available.
- Use Mermaid diagrams only when they clarify architecture or data flow.
- Use `No changes.` for unchanged APIs, data models, persistence, or configuration.
- Use `Not applicable.` for sections that do not apply to the design.
- Keep open questions explicit and owned when possible.

## Verification Checklist

Before reporting completion, confirm:

- Frontmatter exists and includes `doc_id`, `doc_type: design`, `title`, `status`, `tags`, `code_paths`, `related`, `last_updated_at`, and `summary`.
- Required sections are present.
- Optional sections are either relevant, omitted intentionally, or marked `No changes.` / `Not applicable.`
- No placeholder text like `<scope item 1>`, `TODO`, or `TBD` remains unless the user explicitly requested a template-only document.
- The default location is `docs/designs` unless the user specified another folder.
- The document has design substance: goals, non-goals, boundaries, decisions, alternatives, invariants, error behavior, testing strategy, and related docs.
