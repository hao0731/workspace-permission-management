---
name: checking-design-doc-drift
description: Use when reviewing a Git repository, branch, or PR for divergence between design documents, architecture notes, ADRs, specs, plans, README docs, and code implementation; especially when checking stale docs, doc-code drift, freshness, implementation parity, or whether AI agents can trust design context.
---

# Checking Design Doc Drift

## Overview

Find semantic mismatches between design documents and implementation. Prioritize drift that would mislead future implementers, reviewers, or AI agents.

## Workflow

1. Scope the review: whole repo, current branch, PR diff, or named docs. If unclear, ask one focused question.
2. Inventory design context: `README*`, `AGENTS.md`, `docs/**`, `adr/**`, `architecture/**`, `design/**`, `spec/**`, `plans/**`, plus filenames containing design, architecture, adr, spec, plan, proposal, rfc, or decision.
3. Record document status: draft, proposed, accepted, implemented, deprecated, and dates. Planned work is not drift unless presented as current truth.
4. Extract concrete claims about behavior, APIs, schemas, routes, configs, commands, dependencies, security, workflows, and ownership.
5. Trace each claim to code evidence: source, tests, schemas, configs, route tables, generated contracts, and build metadata. Prefer executable artifacts over comments.
6. Report findings first, ordered by severity, with document and code citations. Label missing evidence as unverified instead of inventing drift.

Useful commands:

```bash
rg --files -g 'README*' -g 'AGENTS.md' -g 'docs/**' -g 'adr/**' -g 'architecture/**' -g 'design/**' -g 'spec/**' -g 'plans/**'
git diff --name-only <base>...HEAD
git diff <base>...HEAD -- docs README* AGENTS.md
```

Adapt globs to the repo. A missing path is not evidence of drift.

## Severity

| Severity | Use for |
| --- | --- |
| Critical | Security, auth, data integrity, destructive behavior, or public API guarantees contradicted by code. |
| High | Removed entrypoints, wrong architecture, renamed modules, obsolete workflows, or behavior no longer implemented. |
| Medium | Partially stale docs, missing implemented constraints, or edge cases described differently from code. |
| Low | Stale names, examples, commands, paths, screenshots, or minor wording. |
| Not drift | Explicit roadmap/proposal content, deprecated docs, or code changes with no design-doc claim. |

## Output

When drift exists:

```markdown
**Findings**
- [Severity] [Short title]
  Doc claim: [file:line]
  Code evidence: [file:line]
  Risk: [why this misleads work]
  Suggested fix: [update docs, update code, or clarify status]

**Coverage**
- Reviewed docs: [...]
- Reviewed implementation surfaces: [...]
- Unverified areas: [...]
```

When no drift is found, state that directly and still list reviewed docs, implementation surfaces, and residual risk.

## Red Flags

| Mistake | Correction |
| --- | --- |
| Timestamp-only freshness checks. | Compare claims to implementation. |
| Keyword-only grep. | Follow renamed concepts, exports, tests, configs, and contracts. |
| Every missing doc becomes drift. | Report missing docs only when a doc or convention creates the expectation. |
| Difference without impact. | Explain how it misleads implementation or review. |
| Comments as proof. | Prefer code, tests, schemas, configs, and public interfaces. |
| Ignoring doc status. | Draft/proposed docs can intentionally differ from accepted/current docs. |

## Checklist

- [ ] Scope is explicit.
- [ ] Docs and code surfaces were both inspected.
- [ ] Each finding cites a document claim and code evidence.
- [ ] Planned, draft, accepted, implemented, deprecated, and current docs are separated.
- [ ] Findings are severity-ranked by implementation risk.
- [ ] Recommendations say whether to update docs, update code, or clarify status.
