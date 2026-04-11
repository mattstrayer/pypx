# Stats Period Filter

Add a time period selector to the package stats page so users can toggle between 4-week, 3-month, and 6-month views. All filtering and aggregation happens server-side; the frontend sends a `period` query param and re-renders.

## API

**Endpoint:** `GET /api/packages/{name}/stats?period=4w`

**`period` values:**

| Value | Date window | Weekly buckets | Python/System top-N |
|-------|------------|----------------|---------------------|
| `4w`  | last 28 days | 4 | 8 / all |
| `3m`  | last 90 days | 12 | 8 / all |
| `6m`  | last 180 days (all available) | 24 | 8 / all |

Default: `4w`. Invalid values fall back to `4w`.

**Response additions:**

```json
{
  "period": "4w",
  "date_range": { "from": "2026-03-14", "to": "2026-04-10" },
  "overall": [...],
  "python_versions": [...],
  "systems": [...]
}
```

- `period` echoes the applied period.
- `date_range.from` / `date_range.to` are the actual bounding dates of the filtered data (ISO 8601).

**Cache key:** `stats:{name}:{period}` (e.g. `stats:django:4w`).

**Filtering logic:** Before aggregation, discard any `DataPoint` whose `date` falls outside the window. The window is computed as `now - duration` through `now`. Then apply existing `aggregateByWeek` / `aggregateByCategory` with the bucket count from the table above.

## Frontend

**Toggle row** above the charts, inside `PackageStats.vue`:

```
[ 4 weeks ]  [ 3 months ]  [ 6 months ]
```

- Pill-style buttons using existing zinc palette.
- Active state: `bg-zinc-700 text-zinc-100`. Inactive: `text-zinc-500 hover:text-zinc-300`.
- Selecting a period updates a `ref`, which triggers a re-fetch.

**Date range subtitle** below each section header:

```
DOWNLOAD TRENDS
Mar 14 - Apr 10, 2026
```

Formatted as `MMM DD - MMM DD, YYYY`. If the range spans years, show year on both dates.

**Data fetching:**
- `fetchStats(name, period)` adds `?period=` query param.
- `useAsyncData` key becomes `stats-${name}-${period}` so Nuxt caches per period.
- Show a subtle loading indicator (existing spinner) while refetching on period change.

**Type changes (`api.ts`):**
- `StatsData` gains `period: string` and `date_range: { from: string; to: string }`.
- `fetchStats` signature becomes `(name: string, period?: string)`.

## Testing

**Backend (TDD, `stats_test.go`):**
- `TestStatsPeriodFiltersData`: mock data spanning 180 days, request `?period=4w`, verify only last 28 days included in aggregation.
- `TestStatsPeriodDefaultsTo4w`: omit period param, verify same behavior as `4w`.
- `TestStatsPeriodInvalidFallback`: pass `period=bogus`, verify fallback to `4w`.
- `TestStatsDateRangeInResponse`: verify `period` and `date_range` fields present and correct.

**Frontend (Vitest, if component tests exist):** not required for this iteration; the backend tests cover correctness. Manual verification on staging.
