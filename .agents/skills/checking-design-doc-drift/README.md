# Check Design Doc Drift

Use this skill to review whether design documents still match the implementation in a Git repository. It is intended to keep design docs, ADRs, specs, plans, README files, and other agent-facing context trustworthy.

## What It Checks

- Documented behavior versus actual code behavior
- Architecture or module descriptions versus current source layout
- API, route, schema, config, command, and dependency claims
- Status mismatch, such as docs saying a proposal is implemented when code disagrees
- Stale paths, renamed components, removed entrypoints, or obsolete workflows

## Recommended Prompts

```text
Use $checking-design-doc-drift to review this repository for stale design documents.
```

```text
Use $checking-design-doc-drift to compare docs/plans/2026-01-24-oidc-login-design.md with the current implementation.
```

```text
Use $checking-design-doc-drift to inspect the current branch against main and report design-doc/code drift.
```

## Expected Output

The skill should report findings first, ordered by severity. Each finding should include:

- the document claim
- the code evidence
- the implementation or review risk
- a suggested fix

If no drift is found, the result should still state the reviewed docs, reviewed implementation surfaces, and residual risk.

## Usage Notes

- Prefer semantic evidence over timestamps.
- Do not treat every undocumented code path as drift.
- Treat draft, proposed, accepted, implemented, and deprecated docs differently.
- Ask for a base branch or target document when the review scope is ambiguous.
