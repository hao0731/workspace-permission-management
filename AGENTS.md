# AGENTS.md

This document defines how AI agents should operate in this repository and which policy to follow based on task type.

## Core Principles
- Before writing documentation, code, tests, refactors, or review suggestions, first classify the task as Frontend, Backend, or Full-stack.
- All architecture decisions (naming, layering, data flow, dependencies, and boundaries) must align with the relevant policy.
- If a task spans both frontend and backend, follow each policy for its respective part and keep integration points consistent (API contracts, payload schema, and error format).
- Prefer the existing repository structure, naming, and dependency direction over introducing new patterns.
- Do not introduce new dependencies, global state, public APIs, cross-layer imports, or architecture exceptions without an explicit rationale and trade-off.

## Policy Routing Rules

### Backend Development
Treat a task as backend work if it includes any of the following:
- API, service layer, data access layer, database schema, or migrations
- Validation, authentication/authorization, business logic, background jobs, or event handling
- Backend tests, backend documentation, or backend refactoring

**Required reference:**
- [Backend Architecture Principle](docs/policies/backend-architecture-principle.md)

### Frontend Development
Treat a task as frontend work if it includes any of the following:
- UI components, pages, routing, state management, or frontend data flow
- Design system implementation, interaction behavior, usability, or frontend performance
- Frontend tests, frontend documentation, or frontend refactoring

**Required reference:**
- [Frontend Architecture Principle](docs/policies/frontend-architecture-principle.md)

### Design / Plan Documentation
Treat a task as design/plan documentation work if it includes any of the following:
- Creating or updating files under `docs/designs/`
- Creating, updating, finalizing, or moving implementation plans under `docs/plans/active/` or `docs/plans/completed/`

**Required reference:**
- [Design and Plan Docs Policy](docs/policies/design-and-plan-docs-policy.md)

### Policy Documentation
Treat a task as policy documentation work if it creates or updates files under `docs/policies/` or this `AGENTS.md` file.

**Required references:**
- The policy being changed
- Any policy that depends on or routes to the changed policy

## Execution Workflow (Agent Checklist)
1. Classify the task: Frontend / Backend / Full-stack / Design-Plan Docs / Policy Docs.
2. Read and summarize the relevant policy before implementation.
3. Identify the intended layer, module, or document section before editing, and explain why the change belongs there.
4. Ensure all deliverables (docs and code) align with that policy.
5. If there is ambiguity or conflict:
   - Report the conflict and provide options.
   - Avoid implementations that violate policy until clarified.
6. For bug fixes, add or update a test that reproduces the bug before or alongside the fix unless it is infeasible; if infeasible, explain why.
7. Before completion, run the relevant verification commands for the changed area.
8. If verification cannot be run, report the exact command skipped, the reason, and the residual risk.
9. In your response, briefly state which policy (or policies) you followed and which verification was performed.

## Full-stack Tasks
- Follow the frontend policy for frontend changes.
- Follow the backend policy for backend changes.
- Explicitly document API contracts (field naming, error shape, pagination, versioning) and ensure frontend-backend consistency.
- Keep request/response DTOs, frontend Zod schemas, and backend transport DTOs consistent.
- Treat API contract changes as both frontend and backend changes, even if only one side is edited in the current task.

## Verification Requirements
- Prefer repository-provided verification commands when they exist, such as package scripts, Make targets, or task runner commands.
- Frontend changes should be verified with linting, type checking, tests, and build checks when available.
- Backend changes should be verified with `go test ./...` and any available lint, vet, race, or integration checks relevant to the change.
- Documentation-only changes should be checked for internal link correctness, policy consistency, and stale references.
- Do not claim a change is complete, tested, or passing unless the relevant command output was observed.

## Output Requirements
- Produced documents/code must be traceable to the applicable policy.
- Architecture decisions should include a short rationale and trade-off summary.
- Responses must include the policy followed, verification performed, and any skipped checks.
- Any intentional architecture exception must be called out with rationale, impact, and follow-up risk.
