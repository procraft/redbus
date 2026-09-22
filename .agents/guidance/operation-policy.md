# Team operation and approval policy

This is the shared user-approved policy for coordinators and all repo-bound agents, including
standalone checkouts. It takes precedence over older repository/skill instructions about routine
approvals and automatic local verification. Platform sandbox restrictions still apply: standing
authorization is not a reason to bypass a denied tool or use broader credentials.

## Standing authorization

- **YouTrack reading:** read known tickets, comments, linked tickets and attachments whenever useful,
  without asking. User-requested search is authorized too. Ask before initiating a new YouTrack search
  on the agent's own initiative; do not disguise a broad search as reading a known ticket.
- **Git reading:** inspect local/remote history, refs, status and diffs, and fetch without asking.
  Push only after an explicit user request, to the requested repository/branch. Do not ask again for
  the same authorized push. Reading does not authorize reset, cleanup, force-push or discarding work.
  After a confirmed push, sync the task-owned source checkout to the delivered commit so the delivered
  change stops existing as a local diff (see `master-sync.md`, «Leave the source checkout clean»).
- **Local implementation verification:** an explicit implementation, bug-fix, refactoring, cleanup,
  documentation, or conveyor request authorizes the cheapest deterministic local checks needed to
  validate that scoped work: focused regression tests, targeted compile/typecheck/lint, and required
  code or artifact generation. Reuse the current checkout or workflow-owned isolated lease; do not
  create an unrelated database/runtime, run broad suites speculatively, or trigger external side
  effects merely to satisfy a check. Report commands and actual results.
- **Final repository gates and review:** run the proportionate final repo gate only in an explicitly
  requested `review-before-push` or as the bounded gate of an explicitly invoked `fix-ticket` or
  `fix-bug-queue`. Outside those bug workflows, do not start review automatically; a plain push request
  is not review authorization. An explicit run of either bug workflow also authorizes the bounded
  independent review it defines so the lease can reach `ready`. Reuse unchanged focused evidence and
  rerun only checks affected by review corrections. Do not bypass configured Git hooks, and disclose
  checks they execute automatically.
- **Completed-phase reuse:** a later generic `cleanup`, `review`, or `cleanup/review` request means verify
  that the recorded successful phase still matches the same base and complete change identity; it does not
  by itself request another execution. Reuse the result after cheap read-only freshness checks. Repeat the
  phase only when the user explicitly asks for a fresh/rerun pass, the base/scope/change identity or relevant
  assumptions changed, the earlier phase was incomplete or failed, findings remain unresolved, or a different
  requested depth is not already covered. Do not launch another agent, build, or test merely to reproduce
  current evidence.
- **Acceptance/E2E:** browser, API, multi-role, and post-implementation acceptance execution requires
  the user's separate `test-change` decision. Implementation evidence and an internal code review do
  not grant that authorization.
- **Local database reading:** read whenever necessary without asking. Resolve the actual endpoint:
  a localhost tunnel to stage/prod is not a local database. No implicit writes, migrations, fixture
  changes, clones, new databases or permission grants.
- **Production database:** agents do not execute production SQL. Put the statements in a file and
  hand the user one ready command. Any command that writes must use `$SOHOLMS_PROD_PG_RW_URL`
  (role `libicraft`, password entered interactively by the user), never `-U libicraft` on the read
  string: a role inside the URL silently wins over `-U` and `PGUSER`.
  See `.agents/docs/postgres-prod-access.md`.
- **Loki:** read stage logs without asking. Read production logs when the user requested that
  investigation, or ask once before agent-initiated production access. Permission to read logs
  does not authorize database access, changing environments or running a repair.
- **Processes:** inspect processes, ports and readiness without asking. This does not authorize
  killing, restarting or reconfiguring processes belonging to the user or another task.
- **YouTrack comments:** when the user requests a comment, compose and post a concise factual comment
  to the unambiguously identified ticket without asking for approval of the text or repeating the
  posting confirmation. Clarify only missing material facts or an ambiguous target. A draft-only
  request does not authorize posting. Avoid duplicate comments after an uncertain API response.

## Existing E2E coverage during development

For changes to observable LMS behavior or UI structure, check automated-test impact
before implementation and revisit it against the final diff. This is source inspection,
not acceptance execution; skip unrelated documentation or formatting-only changes.

- Keep server-rule and data-integration tests with the owning service (`lms-back`
  for LMS backend behavior), component/interaction tests with their frontend owner,
  and new LMS browser scenarios in `lms-front/e2e/`.
  Choose the cheapest level that proves the expected behavior; retain E2E for
  outcomes requiring the real UI, API and data to agree. A mocked component test
  does not establish backend permissions or end-to-end compatibility.
- Resolve the task's actual frontend and legacy `procraft/lms-test1` checkouts.
  A runtime slot called `lms-test` is not the legacy test repository. Do not assume
  developer-specific absolute paths or choose unrelated worktrees. If unavailable,
  report coverage as uninspected, not absent; ask only if its location blocks work.
- In a frontend checkout containing `e2e/`, read its nearest `ReadmeAI.md`, run guide
  and coverage catalog, then inspect active scenarios and related component tests.
  During migration also read legacy `AGENTS.md`, `docs/test-catalog.md` and
  `docs/development-workflow.md`. Search routes, `data-testid`, UI labels and domain
  actions, then follow Page Object callers to their active assertions. For backend
  changes, follow affected user outcomes rather than matching Scala filenames.
- Keep UI and its new browser-test changes in the same frontend task checkout.
  Legacy coverage remains required until the catalog maps every active old outcome
  to an implemented replacement with successful relevant execution evidence.
  Password login does not replace phone/code coverage; a pilot's `full` selection
  does not mean the migration or the whole platform has been fully checked.
- Preserve selectors and behavior contracts when the product behavior is unchanged.
  If the intended change breaks them, update the owning Page Objects and assertions
  in the same task's test checkout; coordinate a companion change when that checkout
  is unavailable or shared with another task. Do not weaken assertions, add `xit`,
  or retain incorrect product behavior merely to keep an old test green.
- At handoff, include a brief E2E impact result for relevant changes: selected specs,
  what their active assertions actually check, necessary companion edits, and whether
  coverage is direct, partial, absent or unavailable. Suggest the smallest suitable
  complete scenario(s), with a command from the owning checkout and required
  environment/roles/data effects. A related filename or disabled step is not coverage;
  mention known blockers and missing assertions instead of claiming a regression test.
  If nothing fits, state the gap without proposing the full suite by default.
  Select the run scope from consequences, breadth and uncertainty; shared auth,
  permissions, payment and data-contract changes require broader coverage than a
  local presentation change. Unknown impact cannot justify a narrower selection.
- A suggested command is not a completed check. Keep the existing acceptance/E2E
  authorization boundary: propose execution after the fix, and execute only within
  the user's testing decision, reusing any still-applicable authorization. This policy
  does not authorize automatic fixture preparation, CI execution or registration
  of the legacy test repository in the `wt` lifecycle.

## Implementation discipline

For nontrivial implementation, **decompose before editing and execute each slice through an
appropriate agent when delegation is available**. The task coordinator owns this decomposition;
a worker already assigned a matching slice executes it directly, rather than delegating it again.
Nontrivial means multiple responsibilities,
unresolved design, or meaningful contract, persistent-state, authorization or side-effect risk;
line count is not the criterion. Keep a compact execution record in the current task or existing
plan: outcome, owner role and exact paths, dependencies, preserved invariants/acceptance criteria,
focused verification, and the owner of integration. Resolve a design unknown with a bounded
investigation slice before committing dependent implementation to a guessed design.

Reuse the same agent for coherent dependent work. Parallelize only independent slices with
nonoverlapping write ownership, settled interfaces and available build/runtime resources; cap
active agents to useful independent work and available slots. A file or checklist step is not a
reason for another agent. The coordinator owns the execution graph; workers request another
slice rather than recursively spawning their own teams. Keep bounded independent checks already
defined by an explicitly invoked workflow (such as `fix-ticket`); account for their reviewer in
the same resource budget. For a trivial change, use the owning repo's normal route without extra
decomposition agents. When delegation is unavailable, retain
the same outcome/verification record and execute sequentially under the relevant repo rulebook;
report this fallback without blocking otherwise authorized work.

Before adding a policy, stored state, helper/abstraction or dependency, and after a meaningful
slice, check the existing owner and callers: can reuse, derivation or deletion solve the need?
Justify added complexity by today's behavior or a protected boundary, not speculative reuse.
Unify code when it expresses the same rule and changes together; preserve separate owners when
similar code has different reasons to change. Prefer clear contracts and fewer states over merely
fewer lines. Where useful and proportionate, encode durable invariants in types/immutable state,
module boundaries, constraints or focused tests instead of relying only on prose. Record only
consequential decisions; this is implementation work, not an automatic final review.

Close each slice with criterion-to-evidence links: changed paths, what the focused check proves,
command/result, and the checked commit plus identifiable worktree changes (or equivalent artifact
snapshot). Mark inspection-only and unverified criteria explicitly. After later edits, reuse
checks only if their relevant code, inputs and generated artifacts are unchanged. The integration
owner confirms the combined outcome and producer/consumer seams against the final scoped change;
agent completion alone does not prove integration. Keep acceptance execution, final review,
runtime access, commit and push within their existing authorization boundaries above.

## Performance-risk discipline

Always consider performance alongside correctness, simplicity and delivery cost. Prefer the simplest design
that meets the task's user and operational needs. Scale investigation to absolute impact, frequency, affected
tenants/shared resources, uncertainty and recoverability. A low-impact change needs no separate report or
benchmark; a brief rationale in the existing plan/review is enough. Technical categories and slowdown ratios
are investigation signals, not automatic gates.

For a plausible material risk, record what improves, what must remain acceptable, the representative workload
and known configuration, and the cheapest check that can decide the trade-off. Use existing objectives or
operational deadlines; distinguish a baseline from a required budget and label unknowns rather than inventing
an SLO. Count the whole operation, including queries, fan-out, commits, lock duration, retries and fixed delays
across batch fragments. The integration owner keeps one cost assessment in the existing plan/contract; repo
owners contribute their part. Update it when cost, scale or assumptions change, and verify it against the final
implementation during review. Reuse still-applicable evidence.

A calculation, query plan or focused test may settle the decision; measure representative behavior when
contention or composition makes that bound unreliable. Functional checks alone do not establish throughput.
Block demonstrated material user/operational harm, a violated agreed budget, or a reachable uncontrolled cost
increase without effective mitigation. A slowdown within the budget can be an acceptable price for simplicity
or correctness: state the trade-off, and stop when sufficient evidence supports it. Do not optimize merely to
beat the baseline or continue checks that cannot change the delivery decision.

Bounded residual performance uncertainty may be handled by an already available, authorized limited rollout
with an owner, observable stop threshold and effective disable/rollback path. Record these briefly where the
risk warrants it; do not require new flags, metrics or a rollout framework for every change. A tenant-limited
release can still affect a shared database. Code rollback cannot undo deleted data, exposed tenant information
or completed external payments: check those invariants before release. Missing safeguards or evidence remain
explicit gaps; propose the smallest check when they prevent a delivery conclusion. Existing runtime, acceptance
and deployment authorization boundaries still apply.

## Coordinating execution

The coordinator passes this policy and the current authorization scope to every delegated agent.
Reuse a current ticket snapshot and environment diagnostics; do not make each agent fetch them again
unless fresher evidence is needed. One agent owns shared runtime/DB diagnostics at a time.

Use stable command forms and a tool's working-directory argument instead of changing prefixes with
`git -C`, shell wrappers or arbitrary inline Python. Prefer the standard ticket/attachment scripts.
Run ordinary workspace commands in the sandbox; escalate only when technically necessary. When
escalation is unavoidable, use a narrow reusable rule where safe, not a global shell/SQL allowance.

Generate GraphQL schemas only from the local backend associated with the current worktree. If that
path fails, report it and ask; do not construct another backend/database as a workaround.

## Verification and scope limits

An explicit request to configure/check agent permissions permits read-only configuration inspection,
rule matching, syntax checks and checking synchronization. It is not permission to run unrelated product
tests or final repository gates, access production, create databases or publish a test comment.

OS/tool rules cannot infer user intent or reliably classify arbitrary SQL as read-only. Never claim
that a prefix rule enforces those semantic conditions. See `.agents/docs/agent-permissions.md` in the
coordination workspace for installation, effective-policy checks and remaining technical gaps.
