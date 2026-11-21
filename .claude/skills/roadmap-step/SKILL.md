---
name: roadmap-step
description: Pick up the next unfinished check-uptime roadmap step (or the one the user names) and finish it end to end with tests. Use when the user says "next step", "continue", "finish step N", or asks to work on a roadmap item.
---

# Roadmap step

1. Read "Roadmap status" and "Known issues" in `CLAUDE.md`, and the matching sections of `docs/spec.md`. If the user named a step, take that one; otherwise the lowest-numbered step not marked done.
2. Read the existing code the step touches. Build on it; keep the user's structure and names unless they block the step.
3. If the step needs an answer to an open question in the spec or a decision not listed under "Decisions" in `CLAUDE.md`, ask the user before writing code. Otherwise proceed.
4. State a short plan (files to add or change, tests to write) in a few lines, then implement.
5. Write the tests the spec asks for in "Non-functional" for that area (table-driven assertions, `httptest` checker, incident open/resolve, store against a temp SQLite DB).
6. Fix any "Known issues" entry that belongs to this step and remove it from the list.
7. Run `/verify`. All stages must pass.
8. Update "Roadmap status" in `CLAUDE.md`, and `docs/spec.md` / `docs/database-schema.md` if behavior or schema changed.
9. Summarize: what was built, what was tested, what is left. Propose a conventional commit message; commit only if the user asks.

Use the area skills when relevant: `/sqlite-store` for step 4, `/ui-feature` for step 6, `/add-assertion` for new assertion types.

Keep each step a coherent, reviewable change. If a step is large (the UI), split it into sub-steps (website CRUD, endpoint CRUD, assertions, dashboard) and finish one per change.
