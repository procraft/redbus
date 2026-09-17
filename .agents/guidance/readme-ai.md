---
description: "ReadmeAI.md: find the owning module document through the index, read it before work, keep it a current-state context card after durable changes"
alwaysApply: true
---

# ReadmeAI.md — module-local context for AI agents

> **Managed downstream file.** When this file is inside a downstream project, neither humans nor
> agents may edit it there. Local changes will be overwritten by the next synchronization. Edit
> `lms-ai-multi-repo/.agents/guidance/readme-ai.md` and run `make shared-agent-guidance` from the
> coordination workspace instead.

This is the shared procedure for working with `ReadmeAI.md`. In SOHO.LMS repositories it is
synchronized from `lms-ai-multi-repo`; change the canonical file there instead of editing a
downstream copy.

## Before substantive work

Load the authorization guidance for the assigned execution profile first, including in standalone
checkouts. General-purpose workers read `.agents/guidance/operation-policy.md`; a
coordinator-classified bounded worker reads `.agents/guidance/agent-baseline.md` and
`.agents/guidance/bounded-worker.md` instead. These boundaries take precedence over older local
test/linter and approval instructions.

1. **Locate the owners through the index.** `ReadmeAI.index.md` at the repository root is one line
   per document: directory, context card, `Sections:` (its H2 titles) and `Also:` (sibling Markdown).
   Grep it for the task's terms — entity, table, GraphQL field, UI section — and note every owning
   directory. Read the whole index only when scoping work that crosses several modules.
2. **Read the nearest document.** Walk upward from the target file to the nearest `ReadmeAI.md`.
   Read its context card and the sections the index matched; read the rest only if the card says the
   document is required as a whole.
3. **Follow routing links.** The context card and «Карта» links name the child, sibling and ADR
   documents that own related rules; open the ones the task touches.
4. The closest document is the most specific source of context. Parent documents provide broader
   constraints and must not be silently contradicted.

## While working

- Apply documented invariants, boundaries, failure modes, and ownership rules in the implementation.
- When documentation and code disagree, inspect history and surrounding code before choosing a side.
- For read-only review, investigation, or planning tasks, propose documentation changes but do not edit
  documentation unless the user asks for changes.

## After a change

Update the deepest relevant `ReadmeAI.md` when the change alters durable knowledge, including:

- business rules or important edge cases;
- architecture, module boundaries, or ownership;
- data flow, persistence, API contracts, or integration behavior;
- non-obvious operational constraints, security invariants, or failure modes.

Edit the section that owns the concept; replace the superseded rule instead of appending an exception
after it. Create a new `ReadmeAI.md` next to the affected code when durable context is missing and the
nearest document would become too broad. Do not manufacture documentation changes for trivial
refactors, formatting, generated output, or facts that are already obvious from the code.

## Document contract

Every `ReadmeAI.md` is a **current-state context card** for one code owner, not a change log.
`make readme-lint` checks the structural part of this contract.

- **H1** names the module. **The first paragraph is the context card**: 2–5 lines before the first
  `##` stating what the module owns, what it explicitly does not own (with a link to the owner), and
  when to read the document. The index shows exactly this paragraph, so write it as a routing rule,
  not as the first interesting fact.
- **Sections are named by concept** («Каноническое членство», «Transaction locks»), never by ticket.
  A ticket id is inline provenance at most: `(SL-12903)`.
- **Content is the current rule**: sources of truth, invariants, non-obvious entry points and flows,
  compatibility, transaction, retry, failure and rollout constraints, and links. State what holds now;
  version control keeps what held before — no «раньше было… теперь…».
- **Rationale lives in an ADR.** A decision with real rejected alternatives goes to the workspace
  `docs/adr/` and is linked from the section; the section keeps the one-sentence consequence.
  Measurements and benchmark figures are at most one dated sentence, or belong to the ADR.
- **One owner per rule.** The enforcing side (usually the backend) documents the invariant; a client
  document records only its presentation and compatibility consequences and links to the owner.
- **Size.** 250 lines is the review trigger (lint warning): look for a child directory that owns part
  of the content and move that part into its own `ReadmeAI.md`, leaving a one-line pointer in the
  parent. 400 lines is the cap (lint error). Split only along a real code boundary; a cohesive topic
  without its own directory stays in place and is compressed instead.
- **Sibling Markdown** in the same directory (`README.md`, protocol notes) is listed by the index under
  `Also:`; link it from the context card when it is required reading.
- Product and domain rules follow the repository's documentation language policy; keep technical
  material concise and searchable.

## Index maintenance

The repository-wide `ReadmeAI.index.md` is generated from the H1, context card, H2 titles and sibling
Markdown of every `ReadmeAI.md`. After creating, deleting, or moving a `ReadmeAI.md`, or changing its
H1, context card or section titles, run `make readme-index`; validate with `make readme-index-check`;
run `make readme-lint` for the document contract.

The optional pre-commit integration may run `make readme-index-hook`: it regenerates and stages only
the derived index when staged ReadmeAI files changed, and fails if ReadmeAI changes are only partially
staged. Pre-push hooks and CI must run the check target only; generating at pre-push time is too late to
change the commits being pushed.
