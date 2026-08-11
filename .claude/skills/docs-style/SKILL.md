---
name: docs-style
description: Writing/editing style rules for .md documentation files (docs/*.md, top-level *.md files) and code comments across this repo, keeping them a concise current-state reference instead of a work log. Use whenever adding, rewriting, or reviewing a .md doc, or writing/editing a comment in any source file. Not for spec/README.md (see readme-style) or spec-porting comments in x/sovereignty|x/da|x/vigilante (see go-spec-fidelity, whose citation format this skill defers to).
---

# Docs & comments: write for the reader who arrives cold, not the session that produced them

A `.md` doc or code comment is a **reference for someone who wasn't there** — a future
contributor, a harsh reviewer, or future-you in six months. It describes the system **as it is
now**. It is not a lab notebook, changelog, or transcript of the work session that produced it —
that history already lives in `git log`/`git blame`/PR descriptions, and duplicating it in the
doc only rots the moment the codebase moves past it.

## Hard rules

1. **No session/phase/part markers.** Never write "this session's Part 2", "as of Phase 7 and
   this session's work", "Part 4a", etc. If a fact is true now, state it as a fact. If timing
   genuinely matters (e.g. a migration window), name the real milestone/version — not an internal
   work-session label nobody outside the room can resolve.

2. **No "earlier draft said X, now corrected" narrative.** State the current fact only. `git log`
   on the file is the record of what it used to say; repeating that history in prose is dead
   weight the moment it's written, and actively misleading once a second correction lands and the
   "earlier draft" text itself goes stale. Exception: keep a past-wrong-assumption note only if
   it's a real trap a future reader is likely to independently walk into — judge case by case, and
   default to cutting it.

3. **State the conclusion, not the investigation.** "Docker prefers the network declared second"
   is a fact worth keeping. "Confirmed via `docker inspect` and CometBFT's `/net_info`, tested
   across two attack runs, a partial fix via `gw_priority` was attempted and did not work" is a
   debug transcript — cut to the fact, why it matters, and (if truly needed) one pointer to a
   results file for the full investigation.

4. **Say a thing once.** If two sections would state the same fact, pick the section it belongs
   to and cross-reference (`see §N`) from the other, rather than repeating prose. Duplicated facts
   drift out of sync silently.

5. **Tables/diagrams for structured data, prose for explanation.** Don't narrate a list of
   ports/IPs/services in paragraph form when a table says it in a tenth of the words. Don't format
   a linear causal explanation as a table.

6. **Code comments: extend this repo's existing anti-task-reference rule to timing too.**
   `CLAUDE.md` already bans referencing "the current task, fix, or callers" and issue numbers in
   comments — the same instinct applies to session/phase timing. Don't write "added this
   session", "as of Part 3", "new in this PR" in a comment. State what the code does or why it's
   shaped that way; let `git blame` answer *when* and *by whom*.

7. **Cite precisely, argue briefly.** File paths, function/type names, and (per
   `go-spec-fidelity`/`readme-style`) TLA+ operator + line ranges are load-bearing and must stay
   exact. The prose connecting those citations should be as short as the fact allows — one clause
   of "why" if the reason is non-obvious, nothing if it isn't.

## Before submitting an edit to a .md doc or a comment

- Grep the text you wrote for "session", "Part 1"/"Part 2"/etc., "earlier draft", "previously",
  "this session's" — any hit is a candidate for deletion or rewrite into a plain present-tense
  fact.
- Read it as someone who joined the project today: does every sentence teach them something about
  the system, or does some of it just narrate how the doc came to be?
- If a fact appears twice, delete one occurrence and add a cross-reference instead.
