# Bounded worker profile

Use this profile only after the coordinator has classified the slice as bounded under
`execution-routing.md`. It is a low-context execution profile, not a smaller coordinator.

## Required input

The handoff must contain all of the following. Stop and request a corrected handoff when one is
missing:

- one outcome and one owning repository;
- the absolute checkout path through `lms-ai-multi-repo/repos/<name>/`;
- exact write ownership or an explicit read-only assignment;
- preserved invariants and an acceptance criterion;
- one focused verification command or an explicit inspection-only result;
- the canonical role path and only the additional context files required for this slice;
- the current authorization scope and explicit stop conditions.

## Context budget

Read only `agent-baseline.md`, the assigned canonical role and its compact shared baseline when the
role explicitly requires one, the owning repository's instructions, the nearest relevant
`ReadmeAI.md`, and context files named in the handoff. Do not preload
`workspace.md`, `delegation.md`, `repository-reference.md`, other repository roles, full ticket
history, raw investigation logs, or a complete feature plan. Ask for one missing source by path
instead of loading a broad document set.

When the client supports isolated subagent context, start clean and rely on the compact handoff;
do not inherit the parent conversation merely for convenience.

## Execution boundary

- Make the smallest defensible change that satisfies the assigned criterion and follows the
  existing local pattern.
- Do not delegate further or start parallel workers.
- Do not change a cross-repository contract, persisted model or migration, authentication or
  authorization, payments, security boundaries, dependencies, architecture, production/stage
  runtime state, or an external integration.
- Stop before editing when the root cause, product decision, owner, contract, or verification is
  unresolved. Stop during execution when the change grows another responsibility or repository.
- Return concise evidence to the coordinator; the coordinator owns integration and any re-routing
  to a general-purpose or deep-reasoning worker.
