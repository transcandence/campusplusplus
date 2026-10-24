## Jira Ticket
- **Parent Task:** SCRUM-XX
- **Subtasks Closed:** SCRUM-XX, SCRUM-XX

## Summary of Changes
- 

## Architecture & Quality Gate Checklist
- [ ] **Branch & PR Title:** Branch follows `type/SCRUM-XX-slug` and PR title contains `SCRUM-XX`.
- [ ] **Layer Boundaries:** No SQL in `handler/`, no business logic in `api-gateway/`, no cross-service DB queries.
- [ ] **Contracts Updated?** If HTTP endpoints or events changed, `contracts/openapi.yaml` was updated **first** and `make gen` was run.
- [ ] **Migrations Included?** Every `.up.sql` has a matching, idempotent `.down.sql`.
- [ ] **ADR Needed?** Architectural choices are documented in `docs/adr/000X-*.md`.
- [ ] **Quality Gates Passed:** `make lint` and `make test` pass locally; no secrets or `.env` files committed.
