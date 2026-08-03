---
name: readme-style
description: Writing/editing style rules for spec/README.md (the TLA+ formal-specification writeup for the Engram Hybrid Adaptive Consensus protocol). Use whenever adding, rewriting, or reviewing content in that file.
---

# spec/README.md writing style

`spec/README.md` is **not** the paper — it is one component of it: the writeup for the TLA+ formal specification and model-checked verification of the protocol (design of the spec, the invariants/properties proved, and the verification results). It is read by reviewers who will check every claim against the TLA+ code in `core/` and every number against a real run. **This standard applies to every section of this file, not only procedural ones** — abstract, problem statement, design, proofs, results, all of it. Optimize for **concise, complete, plain vocabulary** — never for length, and never for sounding more academic than the sentence needs to be. Scope stays to the formal spec and its verification; broader paper content (implementation, evaluation outside the TLA+ model, related work, etc.) lives elsewhere and is out of scope for this file and this skill.

**Reference model:** the Abstract and §1/§2 as currently written are the bar to match — confirmed directly by the user as the right style. What makes them work:
- Short paragraphs, one idea each. No paragraph restates the one before it in different words.
- A claim is stated once, plainly, and moved on from (e.g. "We answer affirmatively.") — not hedged or re-asserted three ways.
- **Bold** marks the single most important sentence in a section, not most of it. If everything is bold, nothing is.
- Sequential or categorical content is a list or a table, never prose trying to do a list's job.
- Precise technical terms are kept (this is a formal-methods paper); what's cut is padding around them — throat-clearing transitions ("It is worth noting that...", "Furthermore..."), triple-hedged qualifiers, and sentences that exist to sound formal rather than to say something.

## Hard rules

1. **Every command must be copy-pasteable and true right now.** Before writing a command block, confirm the file paths it references exist in the repo as named. Do not invent directory structures or file names "for illustration."

2. **Every number carries provenance or is marked stale.** A benchmark ("~100 minutes", "37.7M states") is a claim that it was actually measured. If you did not just measure it, either omit it, or state explicitly that it is stale/pending re-measurement and why (e.g., "predates the symmetry-reduction fix"). Never present an unverified number as current fact.

3. **No placeholders in committed content.** Never write `[TODO]`, `[Điền ...]`, or similar. A table row or claim is either backed by a real result, or it is left out of the committed text until it has one (track it in an issue/task list instead, not in the README).

4. **Every operator, invariant, or property name must match the code.** Grep for it before writing it. A name that doesn't exist in `core/*.tla` (or exists under a different name) is a defect, not a stylistic choice — this has been the single most common real bug found in this file (see `EventualDecisionUnderGST` → `EventualDecisionUnderGSTLiveness`, `Corr` → `HonestNodes`, fictional §5.2 transition operators that were pure pseudocode).

5. **One command block per distinct action.** Don't bundle unrelated steps (e.g., "install X" and "run Y") into a single fenced block just to save vertical space — the reader should be able to copy one block and know exactly what it does.

6. **Explain the "why" only when it is non-obvious.** A sentence justifying an unusual design choice, a tool limitation, or a counter-intuitive number earns its place — in the architecture/methodology sections (e.g. §3, §10.1) where design rationale belongs. A sentence restating what the adjacent code or table already shows does not earn its place anywhere — cut it.
   - **"How to run X" sections get zero rationale, full stop.** They exist so a reader can copy a command and get a result. Confirmed by direct user feedback: a first pass that kept "why" paragraphs (tool-limitation rationale, directory-layout history, a footgun anecdote) next to the Apalache run commands was still judged too long; cutting all of it to prerequisites → command → expected output was the right call.

7. **Uniform shape for parallel sections.** Every "how to run X" section follows: Prerequisites → Command → Expected outcome → (optional) notes/troubleshooting. A reader who has read one such section should be able to skim the next by pattern-matching structure, not re-reading prose.

8. **Tables for comparative data, prose for explanation, code blocks for verbatim commands or spec excerpts.** Don't narrate a table's contents in prose above it beyond a one-line intro; don't format a linear explanation as a table.

9. **Cross-reference, don't duplicate.** If a concept is explained in full elsewhere in the document (e.g., the four-layer refinement hierarchy), point at the section number rather than re-explaining it.

10. **State machine / invariant names, file paths, and section numbers are load-bearing text — treat them like code.** A rename in `core/*.tla` or a renumbered section is a README bug until fixed here too.

## Before submitting an edit to this file

- Grep every backticked identifier you introduced or touched against `core/*.tla` to confirm it still exists under that exact name.
- Re-read every number you did not personally just measure in this session — mark it stale or remove it if you can't vouch for it.
- Read the edited section once as if you were a reviewer who will run the commands verbatim — would every one of them work, unmodified, from a clean checkout at the stated prerequisites?
