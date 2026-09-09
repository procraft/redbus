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
- **Acceptance/E2E:** browser, API, multi-role, and post-implementation acceptance execution requires
  the user's separate `test-change` decision. Implementation evidence and an internal code review do
  not grant that authorization.
- **Local database reading:** read whenever necessary without asking. Resolve the actual endpoint:
  a localhost tunnel to stage/prod is not a local database. No implicit writes, migrations, fixture
  changes, clones, new databases or permission grants.
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
