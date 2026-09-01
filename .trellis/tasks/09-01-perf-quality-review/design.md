# Design: Full Repository Code Quality Audit

## Review Architecture

The audit is organized by ownership boundary, then recombined through cross-layer contract passes. This prevents a long flat list of local comments and makes structural simplifications visible.

### Audit domains

1. **Backend runtime** — composition root, HTTP boundary, cache/refresh lifecycle, JW gateway, token manager, models, errors, logging, metrics, and embedding.
2. **Frontend runtime** — API normalization, SWR orchestration, selection/time state, component boundaries, native/Ant Design composition, CSS ownership, and bundle configuration.
3. **Operations and release** — installer transaction, persisted configuration, systemd/Nginx rendering, release scripts, workflows, Taskfile, and operator documentation.
4. **Quality infrastructure** — tests, fakes, coverage of risky paths, command parity, security/audit gates, generated artifacts, and repository hygiene.
5. **Cross-layer contracts** — config flow, timeout/retry budgets, API JSON parity, public model consumption, frontend embedding, and release asset layout.

## Review Passes

### Pass 1: Inventory and size boundaries

- Enumerate tracked, project-owned source/test files.
- Record line counts and explicitly analyze every project-owned file over 1,000 lines.
- Classify generated/managed files separately so they do not create false decomposition findings.

### Pass 2: Architecture and ownership

- Map each subsystem's canonical owner and public boundary.
- Identify feature logic leaking into shared paths, duplicated policy, package drift, thin wrappers, or unnecessary exported APIs.
- Ask whether a different ownership boundary deletes branches or state synchronization.

### Pass 3: Control flow and state

- Trace concurrency, retries, fallback variants, stale/partial data, lifecycle shutdown, React derived state/effects, installer transaction state, and CI/release orchestration.
- Look for half-applied state, hidden coupling, lock-order assumptions, serialized independent work, and special cases inserted into busy flows.

### Pass 4: Type and data boundaries

- Trace JW payload → backend model → response envelope → frontend normalization → render model.
- Trace environment/install state → systemd env → `config.Load` → constructors.
- Compare manual JSON or shell parsing paths with canonical typed/standard-library alternatives.

### Pass 5: Tests and executable evidence

- Inspect tests around every candidate blocker/high finding.
- Run broad offline checks and targeted probes for suspicious fast paths or parity assumptions.
- Treat exploratory agent claims as hypotheses until the main audit verifies the exact code and behavior.

### Pass 6: Consolidation

- Merge duplicate symptoms under the structural root cause.
- Prefer one high-value finding describing a simpler model over many comments on each branch produced by that model.
- Produce a final approval decision and a recommended remediation order.

## Severity and Approval Model

- **Blocker** — clear correctness/security/data-loss/atomicity issue, unjustified project-owned file over 1,000 lines with material risk, or a structural regression with an obvious simplification path that should precede approval.
- **High** — substantial maintainability or boundary problem likely to cause defects, drift, or expensive future changes.
- **Medium** — concrete design debt or missing verification with bounded current impact.
- **Low** — worthwhile but non-blocking cleanup; included only when it adds signal.

Approval requires no blockers, no clear structural regression, no unjustified >1,000-line project-owned file, no obvious spaghetti growth, and no missed code-judo simplification that materially reduces the model.

## Evidence Contract

Each blocker/high finding must include:

1. exact `file:line` anchors;
2. traced inputs/state and affected outputs;
3. the violated repository contract or quality principle;
4. evidence from tests/specs/commands where applicable;
5. a concrete preferred remedy;
6. a statement of whether the finding is a verified defect or structural risk.

Possible false positives are actively challenged against actual data sources, comments, tests, and `.trellis/spec/` before inclusion.

## Parallelism and Integration

Read-only audits of backend, frontend, and operations can run in parallel. Cross-layer analysis and final severity decisions remain centralized so separate reviewers do not produce contradictory ownership recommendations or duplicate findings.

## Output

The sole task deliverable is `review.md`, structured as:

1. approval verdict;
2. prioritized findings;
3. code-judo opportunities;
4. file-size/decomposition assessment;
5. verification results;
6. reviewed scope and explicit exclusions;
7. recommended remediation order.

No product source is changed. If the user later requests fixes, findings should be converted into independently verifiable implementation tasks rather than bundled into this audit.

## Risks and Mitigations

- **Audit breadth dilutes depth** — use domain passes and prioritize high-risk execution paths.
- **AI false positives** — independently re-read and verify every significant claim.
- **Tests passing masks structural debt** — approval includes maintainability and decomposition, not correctness alone.
- **Platform-dependent checks** — record exact skipped commands and reason; use closest portable equivalent where safe.
- **Review scope expands into managed tooling** — distinguish project product code from generated/Trellis harness code while still checking integration boundaries.
