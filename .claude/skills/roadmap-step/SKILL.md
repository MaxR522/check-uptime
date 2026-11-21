---
name: roadmap-step
description: Pick up the next unfinished check-uptime roadmap step (or the one the user names) and finish it end to end with tests. Use when the user says "next step", "continue", "finish step N", or asks to work on a roadmap item.
---

# Roadmap step

1. Read `docs/roadmap.md` and pick the step: the one the user named, otherwise the first with status `todo` or `doing`. From then on read only what that step lists under **Read** (plus the spec sections it names). Do not open files outside that list unless the step cannot be done without them.
2. Read the existing code the step touches. Build on it; keep the user's structure and names unless they block the step.
3. Ask the step's **Ask** questions first. If the step needs an answer to another open question in the spec or a decision not listed under "Decisions" in `CLAUDE.md`, ask the user before writing code. Otherwise proceed.
4. State a short plan (files to add or change, tests to write) in a few lines, then implement.
5. Write the tests the spec asks for in "Non-functional" for that area (table-driven assertions, `httptest` checker, incident open/resolve, store against a temp SQLite DB).
6. Stay inside the step's **Touch** list; note anything found out of scope under the right later step in `docs/roadmap.md` instead of fixing it.
7. Run `/verify`. All stages must pass.
8. Set the step status to `done` in `docs/roadmap.md` (add a **Notes** line only if later steps need it), update "Next" in `CLAUDE.md` "Roadmap", and `docs/spec.md` / `docs/database-schema.md` if behavior or schema changed.
9. Summarize: what was built, what was tested, what is left. Propose a conventional commit message; commit only if the user asks.

Load only the area skill the step names (`/sqlite-store`, `/ui-feature`, `/add-assertion`).

Keep each step a coherent, reviewable change. If a step turns out too big for one session, stop at a green `/verify`, mark it `doing`, and split the remainder into a new step in `docs/roadmap.md`.
