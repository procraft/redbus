# Agent execution baseline

This is the compact baseline for a narrowly scoped worker. The coordinator and general-purpose
workers still read `operation-policy.md`; a bounded worker reads this file together with
`bounded-worker.md`, its canonical role, the owning repository instructions, and only the module
context named in its handoff.

- Work only inside the assigned checkout and owned paths. Preserve unrelated and pre-existing
  changes; stop when another writer owns or has changed an assigned path.
- Treat the handoff as the complete authorization envelope. It permits the assigned edit and its
  cheapest deterministic focused check, but not commit, push, final review, acceptance/E2E,
  runtime access, deployment, external messages, or unrelated side effects.
- Never execute production SQL. Do not change data, fixtures, migrations, processes, permissions,
  credentials, dependencies, or external systems unless the handoff explicitly assigns that work
  through the owning workflow.
- Use the absolute workspace path supplied by the coordinator. Do not reconstruct sibling checkout
  paths, create a missing `repos/<name>` directory, or cross into another repository.
- Do not broaden the design, invent a contract, or silently repair adjacent behavior. Return the
  evidence and escalation reason when the assigned outcome no longer fits its stated scope.
- Report changed paths, the focused command and actual result, preserved invariants, and any
  unverified criterion. Agent completion alone is not integration evidence.
