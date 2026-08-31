---
description: "ReadmeAI.md: read the nearest documentation before work and update it after durable changes"
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

1. If the repository root contains `ReadmeAI.index.md`, consult it first.
2. Starting from the target file or directory, walk upward and read the nearest `ReadmeAI.md`.
3. If that document points to child or sibling documentation relevant to the task, read it too.
4. Treat the closest document as the most specific source of context; parent documents provide
   broader constraints and must not be silently contradicted.

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

Create a new `ReadmeAI.md` next to the affected code when durable context is missing and the nearest
document would become too broad. Do not manufacture documentation changes for trivial refactors,
formatting, generated output, or facts that are already obvious from the code.

## Writing guidelines

- Describe current behavior and intent, not a chronological change log.
- Explain why a constraint exists when that helps prevent a future regression.
- Keep product and domain rules in the language required by the repository's documentation policy.
- Keep technical and architecture material concise and searchable.
- Prefer one focused document near the code over a large root document with unrelated details.
- Treat roughly 300 lines as a soft threshold: split by responsibility when a document becomes hard to
  scan.

## Index maintenance

The repository-wide `ReadmeAI.index.md` complements nearest-document discovery by exposing relevant
context in sibling subtrees. After creating, deleting, or moving a `ReadmeAI.md`, or changing its H1
or first summary paragraph, run `make readme-index`; validate with `make readme-index-check`.

The optional pre-commit integration may run `make readme-index-hook`: it regenerates and stages only
the derived index when staged ReadmeAI files changed, and fails if ReadmeAI changes are only partially
staged. Pre-push hooks and CI must run the check target only; generating at pre-push time is too late to
change the commits being pushed.
