# Design and Plan Docs Policy

This policy defines the lifecycle, storage locations, status transitions, and maintenance rules for design documents and implementation plan documents in this repository.

## Scope
- This policy applies to any work that creates, updates, or moves:
  - Design documents under `docs/designs/`
  - Implementation plan documents under `docs/plans/active/` and `docs/plans/completed/`

## 1) Design document creation and storage
- Whenever a design document is created, it **must** be stored under `docs/designs/`.
- Design documents should remain traceable and include at least:
  - Background and goals
  - Design decisions and rationale
  - Boundaries and impact scope

## 2) Plan creation, finalization, and commits
- Whenever an implementation plan is created from a design document, it **must first** be stored under `docs/plans/active/`.
- Once an implementation plan is finalized, there **must** be a git commit that records the finalized content in version history.
- Plans should explicitly link to their source design documents (for example, with relative links).

## 3) Transition after implementation is completed
- When implementation is completed, the plan document **must** be moved from `docs/plans/active/` to `docs/plans/completed/`.
- This move **must** be committed via git so the status transition is auditable.

## 4) Ongoing design maintenance and document linking
- If any future change affects an existing design, the relevant design document **must** be updated (not only code or plan files).
- When helpful, design documents **should** cross-reference each other with hyperlinks to make dependencies and impacts explicit.
- Example: if design B changes a section of design A, update that section in A and note its relationship to B.

## 5) Recommended workflow
1. Write or update design docs in `docs/designs/`.
2. Create implementation plans in `docs/plans/active/` and link the related design docs.
3. Commit once the implementation plan is finalized.
4. After implementation is done, move the plan to `docs/plans/completed/` and commit that move.
5. If later requirements affect existing design, update `docs/designs/` and add needed cross-links.

## 6) Compliance checklist
- [ ] Design document is under `docs/designs/`
- [ ] Initial implementation plan is under `docs/plans/active/`
- [ ] Finalized implementation plan has been committed
- [ ] Completed implementation plan has been moved to `docs/plans/completed/` and committed
- [ ] Existing design docs were updated when impacted by later changes
- [ ] Cross-links between related design docs were added when needed
