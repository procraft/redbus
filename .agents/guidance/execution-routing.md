# Model-independent execution routing

The coordinator selects an execution class by uncertainty, responsibility and risk. Shared rules
name capabilities, never vendors or model identifiers; client adapters map each class to an
available model and tool configuration.

## Execution classes

- **Coordinator:** owns requirements, cross-repository contracts, decomposition, integration and
  final synthesis. Use for ambiguous, architectural, security-sensitive or multi-repository work.
- **Repository worker:** owns a coherent slice in one repository when investigation, several
  responsibilities or broader validation may be required. Use the canonical repo-bound role.
- **Bounded worker:** executes one settled, independently verifiable outcome under
  `bounded-worker.md`. Use a client-specific fast adapter when available; otherwise fall back to the
  ordinary repository worker without weakening the rules.
- **Read-only explorer:** answers a delegated search with a conclusion and `path:line` evidence
  under `explore-worker.md`. Client explorer adapters run it below coordinator cost; when a client
  has none, its built-in explorer is used.
- **Independent reviewer/runtime investigator:** use only through the explicit workflow and
  authorization that owns that capability; neither is a bounded-worker shortcut.

## Bounded eligibility gate

All conditions must be true before dispatch:

1. There is one owning repository, one outcome and no unresolved product or design decision.
2. The owner, write paths, preserved invariants, acceptance criterion and focused check are known.
3. The solution follows an established local pattern and needs no new dependency or abstraction.
4. The slice does not touch a cross-repository contract, persisted schema or migration,
   authentication/authorization, payments, security boundary, runtime environment, external side
   effect, or shared infrastructure.
5. The assigned checkout and paths have one writer; parallel work cannot overlap them.
6. Failure can be reported without leaving a partial externally visible state.

Patch size is not an eligibility criterion. A one-line permission change may require a coordinator;
a multi-file mechanical update may remain bounded when its ownership and proof are settled.

## Compact handoff contract

Pass decisions and evidence, not conversation history:

```text
Outcome:
Repository and absolute checkout:
Owned paths:
Preserved invariants:
Acceptance criterion:
Focused verification:
Relevant context files:
Authorization scope:
Stop conditions:
```

The coordinator reclassifies the slice when a worker returns an escalation. Do not tell the same
bounded worker to continue outside the gate merely to avoid a new handoff.
