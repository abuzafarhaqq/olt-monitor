# OLT Diagnostics — frontend (Stage 1)

React 18 + TypeScript 5.6 + Vite 6. Application shell, routing, design tokens,
reusable components, a mock-data Network Overview, Collection Health, OLT
Inventory (with device detail), PON Explorer, ONU Management with per-ONU
diagnostics, Optical Diagnostics, Incidents & Events, History & Reports with
CSV/JSON export, a Diagnostics Console, Users & Access and Settings.

The service layer talks to the Go API through `services/api.ts`; until that API
exists it runs against `services/mockBackend.ts` behind one switch
(`USE_MOCK_API` in `services/client.ts`). No OLT is ever contacted by the
browser, and there is no real login yet.

## Run it

```bash
pnpm install     # or: npm install
pnpm dev         # http://localhost:5173
pnpm typecheck   # tsc --noEmit
pnpm build       # typecheck + production bundle
pnpm lint
```

Node 18.19+ is enough for Vite 6. Vite 7 needs Node ≥ 20.19 — do not bump the
Vite major without bumping Node first.

## What is real, what is mock

Everything on the Overview comes from `src/mock/mockData.ts`, a deterministic
(fixed-seed) fixture. The page header always shows **MOCK DATA — not live**.
Shapes match `OverviewResponse`, i.e. the intended
`GET /api/v1/dashboard/overview` payload, so Stage 2 replaces a data source
rather than a page.

## Layout

```
src/
  app/            App.tsx, router.tsx (declarative react-router), AuthContext.tsx
  services/       api.ts (transport), mockBackend.ts, client.ts (the switch),
                  auth.ts (roles + permissions), sse.ts (live feed)
  hooks/          useLiveEvents.ts
  components/
    layout/       AppLayout, Sidebar, Topbar
    common/       PageHeader, MetricCard, StatusBadge, CollectionBadge,
                  FreshnessIndicator, DataTable, States (empty/loading/error)
    charts/       AvailabilityTrendChart (hand-drawn SVG, no chart library)
  constants/      navigation.ts (single source for sidebar + router),
                  opticalThresholds.ts (per vendor/model, never global)
  features/
    dashboard/    OverviewPage + OltHealthTable, RecentEvents, ProblemPons,
                  selectors.ts (all derived numbers live here)
    olts/         OltInventoryPage, OltDetailPage, AddOltDrawer, selectors.ts
    pons/         PonExplorerPage + selectors.ts (tree, ONU filters, PON metrics)
    onus/         OnuListPage, OnuDetailPage, OnuTable, selectors.ts
    optical/      OpticalDiagnosticsPage + selectors.ts (RX distribution, per-PON health)
    incidents/    IncidentsPage, IncidentDrawer, selectors.ts
    reports/      ReportsPage + selectors.ts (range, comparison, rows)
    console/      DiagnosticsConsolePage (allow-listed commands only)
    users/        UsersAccessPage (roles + permission matrix)
    settings/     SettingsPage (thresholds, intervals, retention, audit)
    collection/   CollectionHealthPage + selectors.ts (job filters, KPIs)
    placeholder/  PlaceholderPage for routes not built yet
  mock/           mockData.ts
  styles/         tokens.css, global.css, layout.css, components.css, dashboard.css
  types/          domain.ts
  constants/      ... reports.ts (report registry)
  mock/           mockData.ts, collectionHealth.ts, onuDiagnostics.ts, history.ts
  utils/          format.ts, export.ts (CSV/JSON)
```

`src/features/` is a sibling of `src/components/` — the spec's ASCII tree in §9
nested it inside `components/`, which contradicts §12.

## Rules this stage already enforces

- **Operational state ≠ collection state.** `OnuStatus` (online / offline /
  unknown) and `CollectionStatus` (successful / partial / failed / stale /
  unsupported) are separate types, separate badges, never merged. A failed
  collection is amber, never red: red means a confirmed outage.
- **Unknown is never folded into offline.** Separate KPI card, separate badge,
  separate trend series.
- **Two clocks.** `DeviceTimestamps` keeps the OLT's clock and our ingestion
  time; a device clock under 2010 is rendered as `(clock suspect)`.
- **Missing ≠ zero ≠ unsupported.** `rxPowerDbm: null` renders `—`; optical
  severity distinguishes `missing`, `unsupported` and `unconfigured`.
- **Thresholds are per `${vendor}/${technology}`**, declared only for pairs we
  have actually seen. A pair without a threshold is `unconfigured`, not healthy.
- **Duplicates are preserved.** The mock contains two identical deregistration
  events with different evidence references, and both are shown.
- **Ranking uses fresh data only.** `getTopProblemPons` drops stale and failed
  PONs before sorting.
- **No universal optical threshold.** `classifyRx` resolves the table by
  `${vendor}/${technology}`; an ONU with no configured pair shows `No threshold`
  in grey, never `OK`.
- **Every measurement is `{ value, supported }`.** `supported: false` means the
  adapter has no command for that metric on that model; `value: null` means the
  command ran and returned nothing. Only supported metrics get a card.
- **Raw evidence is reproducible.** Command, capture time, parser version and
  completeness travel with the text; a 50 % parse shows a warning.
- **A password has no input field.** The Add-OLT form takes a *credential
  reference*; validation rejects an empty one and the form states that nothing
  is saved until the write API exists.
- **The workflow is a graph, not a dropdown.** `ALLOWED_TRANSITIONS` defines the
  legal moves, so the drawer cannot offer a transition the backend would reject.
- **A hypothesis stays a hypothesis.** The suspected PON outage is rendered in a
  purple callout prefixed "not a confirmed root cause", never as a verdict.
- **Incident changes are session-local** and the page says so whenever one is
  made; nothing is written anywhere until the write API exists.
- **Period comparison is equal-length.** The previous window is computed from
  the selected range, so 7 days is never compared with 30.
- **CSV is quoted per RFC 4180** and JSON is exported from the same row objects,
  so the two files can never disagree.
- **ISO dates compare as strings.** Range filtering is `date >= from && date <= to`
  with no parser in the hot path.
- **No free-text command reaches a device.** The console posts a command *id*
  from an allow-list; the adapter maps it to CLI server-side.
- **Hiding a button is not authorisation.** `PermissionGate` controls rendering;
  every endpoint must re-check the caller's role.
- **One switch, one place.** `USE_MOCK_API` decides between fixtures and the API;
  no component knows which is active.
- **Tabular numerals** on every reading, timestamp, counter and ID.
- **A failed collection never rewrites ONU state.** Collection Health counts the
  states left untouched (`stateLeftUnchanged`) and marks those rows amber, not red.

## Deviations from the spec (deliberate)

| # | Spec | Here | Why |
|---|------|------|-----|
| 1 | §7 `CollectionStatus` has no `skipped` | `CollectionStatus` as written; `CollectionJobResult` adds `skipped` | Job results and ONU collection status are different concepts |
| 2 | §9 nests `features/` under `components/` | sibling | §12 treats features as top-level modules |
| 3 | §5.1 "collection failures" | labelled "Unhealthy collections", counts OLTs | Job failures and OLT-level blindness are different numbers |
| 4 | Optical thresholds undefined in Stage 1 | `constants/opticalThresholds.ts` | Model-specific per §5.4/§5.11, so they live in one table |
| 5 | §8 lists 20 components | 12 built | Only what the Overview uses; the rest arrive with their pages |

## Not built yet

The Go backend and OLT collector, real authentication and token handling, the
live SSE endpoint (the client consumes a mock feed of the same shape), global
search, persistence of incident notes and state, OLT detail tabs for performance
/ collection history / configuration, and the 25k-row performance and
accessibility work of stage 8.
