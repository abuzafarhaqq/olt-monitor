# OLT Diagnostics — frontend (Stage 1)

React 18 + TypeScript 5.6 + Vite 6. Application shell, routing, design tokens,
reusable components, a mock-data Network Overview, Collection Health, OLT
Inventory (with device detail) and PON Explorer. No backend, no OLT connection,
no authentication yet.

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
  app/            App.tsx, router.tsx (declarative react-router)
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
    onus/         OnuTable, shared by PON Explorer and OLT detail
    collection/   CollectionHealthPage + selectors.ts (job filters, KPIs)
    placeholder/  PlaceholderPage for routes not built yet
  mock/           mockData.ts
  styles/         tokens.css, global.css, layout.css, components.css, dashboard.css
  types/          domain.ts
  utils/          format.ts
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
- **A password has no input field.** The Add-OLT form takes a *credential
  reference*; validation rejects an empty one and the form states that nothing
  is saved until the write API exists.
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

Auth and roles, global search, REST client, SSE, incident workflow, the
Diagnostics Console, CSV/JSON export, OLT detail tabs for performance /
collection history / configuration / raw evidence, and every page still marked
`S<n>` in the sidebar.
