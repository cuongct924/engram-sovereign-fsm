---
name: "paper-writing"
description: "Academic writing rules and a pre-submission checklist for computer-science research papers (systems, security, distributed consensus, formal methods). Use whenever drafting, rewriting, reviewing, or polishing text in docs/paper/ (LaTeX sections, abstract, captions, tables, bibliography), when responding to reviewer feedback, or when deciding how strongly a result may be claimed."
---

# Writing Computer Science Research Papers

Principles distilled from *Writing for Computer Science* (Zobel), *The Craft of Research* (Booth,
Colomb, Williams), and reviewer feedback received on this paper. Sections I–V are the method;
section VI is the claim-strength discipline; sections VII–IX cover evaluation, related work, and
revision; section X is the checklist to run on every paragraph before calling it done.

## I. Problem Definition and Research Planning

- Narrow a broad topic into specific questions, then into a research problem that fills a gap the
  reader cares about. A gap is not "nobody has done X"; it is "not having X causes consequence Y
  for stakeholder Z". State the consequence.
- Formulate each research question so it is precise, unambiguous, and **falsifiable**: it must be
  possible for an experiment to answer "no". A question that can only be answered "yes" is a
  demonstration, not research.
- Plan backwards from the final paper: for every claim you intend to make, name the evidence that
  would support it, the baseline it must beat, and the experiment that produces it — before running
  anything.
- **Fix success criteria before collecting data** (metric, threshold, oracle, number of trials).
  Criteria chosen after seeing results invite researcher-degrees-of-freedom criticism.

## II. Argumentation

- A research argument is a **claim** supported by **reasons**, each grounded in **evidence**.
  **Warrants** explain why a reason supports the claim; make them explicit wherever a reader could
  doubt the connection.
- Evidence must be accurate, sufficient, representative, and reported at the right level of detail.
  One live run shows that a behaviour *can* occur; it does not characterize how often it occurs.
- Anticipate objections, alternative explanations, and competing designs. Acknowledge each one and
  respond to it — with evidence where possible, with a bounded concession where not.
- Every design choice that a reviewer could replace with a simpler one needs a sentence explaining
  why the simpler one fails (e.g., why a decision must live inside consensus rather than in each
  node's local policy).
- Distinguish **what the system does** from **what it guarantees** and from **what was observed**.
  These are three different kinds of statement and need three different kinds of support.

## III. Organization and Storytelling

- A paper is a narrative that leads the reader from what they know to what you want them to learn,
  not a log of work performed.
- Standard structure: title, abstract, introduction, background/related work, design, analysis or
  formal results, evaluation, discussion, conclusion, references.
- **Abstract** covers five elements: context, specific problem, limitation of current solutions,
  proposed solution, and evaluation results with their key numbers.
- **Introduction** establishes common ground, states the problem and its consequence, states the
  central claim, lists contributions, and says what the paper is *not* (scope boundaries stated once,
  up front, stop reviewers from attacking a strawman version of the work).
- **Contributions** are concrete and checkable ("a quorum-certified health certificate whose
  aggregate is bounded by honest observations"), each pointing to the section that substantiates it.
  Order them by importance; supporting evidence goes last.
- One research question per evaluation subsection; each subsection opens with the question and
  closes with its answer.
- Present limitations as the boundary of a solid result, not as a list that overwhelms the claim.
  If limitations outweigh the argument, the argument needs strengthening, not more disclaimers.

## IV. Style and Clarity

- Be concise; cut sentences that carry no information.
- Prefer simple sentence structures, short paragraphs (one idea each), and direct, familiar words.
- **Old before new**: open a sentence with information the reader already has, and put new or
  complex information at the end.
- Never obscure the main point with vague or abstract wording.
- Do not overuse hedges ("perhaps", "might", "possibly"); they make the argument sound timid. Hedge
  exactly once, where the uncertainty actually lies, and say what would resolve it.
- Use active voice when the actor matters ("TLC found no violation"); make sure the grammatical
  subject can perform the verb ("properties found zero violations" is wrong).

## V. Drafting and Editing

- Start drafting early; writing exposes gaps in the argument and shapes the remaining research.
- Edit from the reader's perspective, top-down: overall structure, then coherence of each section,
  then clarity of each sentence.
- Maintain absolute consistency in terminology, definitions, notation, units, capitalization of
  system states, and reference format. Define a term once and never switch synonyms for it.
- Every number in the text must match its table, figure, and source data file; recompute derived
  numbers (percentages, ratios) rather than copying them.

## VI. Claim Strength Must Match Evidence

Reviewers from formal-methods and systems communities check this first; one overstated claim makes
every other claim suspect.

| Label | Use only for | Example phrasing |
|---|---|---|
| Theorem / Lemma / Proposition | A statement with a complete proof (pen-and-paper or mechanized) | "Lemma 1. If the certificate is well-formed, …  *Proof.* …" |
| Property (model-checked) | TLC/Apalache result; always state the bound | "holds in all reachable states for N = 4, f = 1, MaxRound = 2" |
| Theorem for a configuration | A model-checked result stated with its full scope | "Theorem (N = 4, f = 1). … — checked by `MC_…`" |
| Target property / Conjecture | Specified but not yet established | "the refinement check that would establish this has not completed" |
| Observed | A live or simulated measurement | "in five trials, recovery completed within …" |

- Model checking explores a finite instance; it does not prove a property for arbitrary N. Say
  "bounded model checking", name the bound, and cite the small-scope hypothesis if you rely on it.
- Experimental results are **evidence**, never **guarantees**.
- Use "shows/indicates" only for verified causal statements; use "suggests" when the cause is a
  hypothesis, and name the experiment that would confirm it.
- Never let a per-component result stand in for a whole-system result; say explicitly which one
  you have.

## VII. Evaluation Methodology and Reporting

**Baselines**
- A baseline must be a credible alternative a reader would actually choose, run on the same binary,
  hardware, workload, and fault schedule, differing in one policy only.
- A baseline that fails "by construction" is a sanity check, not a comparison. Say which is which.
- Where a trade-off exists, show both sides (e.g., one baseline that stays live but is unsafe, one
  that is safe but halts).

**Statistics**
- Report n for every number. Prefer mean ± 95% confidence interval over bare means; report the raw
  values when n ≤ 5.
- For proportions of 0% or 100%, give the upper/lower confidence bound (rule of three: with n
  trials and zero events, the 95% upper bound is ≈ 3/n).
- Report effect sizes as a confidence interval on the difference rather than a standardized score
  that explodes with tiny variance.
- Microbenchmarks: iteration count, variance, and what is and is not included in the measured path.

**Design threats to anticipate**
- Carry-over between sequential scenarios: randomize order or run scenarios independently.
- Sampling resolution: the measurement interval must be finer than the phenomenon (block interval,
  transient states).
- Correlated resources: co-located nodes share CPU, clocks, and failure modes; say so and mitigate.
- Agreement is not correctness: identical results across replicas do not show those results are
  right; add an independent oracle.
- Circularity: when the same authors design attacker and detector, include attacks the detector was
  not tuned for, or an optimizing adversary.
- Make sure each defence is actually exercised: an attack the mechanism never had to stop is not
  evidence that the mechanism works.

**Reproducibility**
- Artifact URL, license, and the exact commit/tag that produced the results.
- Hardware, OS, container runtime, resources per node; versions of every dependency, including
  forks and what they change upstream.
- Every fault-injection parameter (delay, jitter, loss, duration, onset time) and every protocol
  parameter used in each experiment.
- Mathematical definition, denominator, and run length for every metric; seeds and generative models
  for any synthetic data.
- Disclose any data excluded, any bug fixed mid-study, and how measurements before the fix were
  handled.

## VIII. Related Work

- Organize by the questions the literature answers, not by paper; a taxonomy tree or a comparison
  table against explicit criteria makes the gap visible.
- Compare against the state of the art, never against your own earlier sketches or internal
  documents.
- State the novelty claim narrowly and precisely once; do not add commentary about avoiding
  overclaiming.
- Every entry in the bibliography must be verified against DBLP or the publisher; never cite from
  memory.

## IX. Responding to Reviews and Revising

- Triage each comment: already addressed in the current draft, needs new work, needs text only,
  or disagreement with a reasoned response. Reviewers often read an older version.
- Fix the central argument (problem, gap, mechanism) before polishing sentences.
- When a reviewer finds an inconsistency, check it against the code and data before rewriting; the
  text, the implementation, or the reviewer may be wrong.
- Never attribute text in the paper to reviewer or advisor feedback; the paper states facts.

## X. Pre-Submission Checklist

Run on every new or revised paragraph.

- **No promotional or emotive adjectives:** only, rigorously, firmly, clearly, substantially,
  modest(ly), subtle, actionable, straightforward, cheap, dwarfs, "missing dimension". Let the number
  speak ("228 bytes", not "only 228 bytes").
- **No meta-commentary about the writing:** "we state explicitly", "we flag", "honestly", "we
  report this honestly", "rather than restate", "to avoid overclaiming", "worth stating plainly".
  State the fact.
- **"real/genuine"** only where the contrast with simulation matters (e.g., "real network noise"
  versus in-process noise).
- **Tense:** completed experiments in the past tense ("produced", "escalated"); descriptions of
  tables and figures in the present tense ("Table 3 shows").
- **Claim strength matches evidence** (section VI): evidence not guarantees; suggests versus shows;
  Theorem only with a proof; model-checked results always with their bound.
- **Run-on sentences:** split any sentence longer than about 40 words, or with nested dashes and
  parentheses, into single-idea sentences.
- **Say it once:** each limitation (open refinement, single host, …) is stated in full in exactly
  one place; elsewhere, reference that section.
- **Numbers scoped correctly:** state which run a number belongs to (e.g., 3.39 million states is
  the FSM safety run alone, not the sum over layers). No illogical comparisons ("accounts for 180×
  the total").
- **Standard terminology:** "bounded model checking" (not "bound-model-checked"); Apalache is
  SMT-based (symbolic), TLC is explicit-state.
- **No internal project information:** effort estimates in weeks, internal team names, debugging
  history.
- **Define before use:** every label (E4, P3, S5, …) and metric (flapping, absorption rate, …) at
  its first occurrence.
- **Consistency:** every number matches its table, figure, and data file; every cross-reference
  points to the right section; no placeholders (`\pending`, TODO) remain.
