<script setup lang="ts">
import type { DataPoint } from "~/types/api";

const props = defineProps<{
  name: string;
}>();

const periodOptions = [
  { value: "4w", label: "4 weeks" },
  { value: "3m", label: "3 months" },
  { value: "6m", label: "6 months" },
] as const;

const period = ref("4w");

const { fetchStats } = useApi();
const {
  data: stats,
  status,
  refresh,
} = await useAsyncData(
  () => `stats-${props.name}-${period.value}`,
  () => fetchStats(props.name, period.value),
);

async function setPeriod(p: string) {
  period.value = p;
  await refresh();
}

function formatDownloads(n: number): string {
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}

function maxDownloads(data: DataPoint[]): number {
  return Math.max(...data.map((d) => d.downloads), 1);
}

function barWidth(downloads: number, max: number): string {
  return `${(downloads / max) * 100}%`;
}

const overallTrend = computed(() => stats.value?.overall ?? []);
const pythonVersions = computed(() => stats.value?.python_versions ?? []);
const systems = computed(() => stats.value?.systems ?? []);

const totalDownloads = computed(() => overallTrend.value.reduce((sum, p) => sum + p.downloads, 0));

const weeklyAverage = computed(() =>
  overallTrend.value.length ? Math.round(totalDownloads.value / overallTrend.value.length) : 0,
);

const peakWeek = computed(() => {
  if (!overallTrend.value.length) return null;
  return overallTrend.value.reduce((max, p) => (p.downloads > max.downloads ? p : max));
});

const dateRangeLabel = computed(() => {
  const range = stats.value?.date_range;
  if (!range) return "";

  const from = new Date(range.from + "T00:00:00");
  const to = new Date(range.to + "T00:00:00");

  const fmt = (d: Date, includeYear: boolean) => {
    const opts: Intl.DateTimeFormatOptions = { month: "short", day: "numeric" };
    if (includeYear) opts.year = "numeric";
    return d.toLocaleDateString("en-US", opts);
  };

  const sameYear = from.getFullYear() === to.getFullYear();
  return `${fmt(from, !sameYear)} – ${fmt(to, true)}`;
});

interface SplinePoint {
  x: number;
  y: number;
}

function buildSplinePath(pts: SplinePoint[]): string {
  if (pts.length < 2) return "";
  let d = `M ${pts[0].x.toFixed(1)} ${pts[0].y.toFixed(1)}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const p0 = pts[i];
    const p1 = pts[i + 1];
    const cp1x = (p0.x + (p1.x - p0.x) / 3).toFixed(1);
    const cp2x = (p0.x + (2 * (p1.x - p0.x)) / 3).toFixed(1);
    d += ` C ${cp1x} ${p0.y.toFixed(1)} ${cp2x} ${p1.y.toFixed(1)} ${p1.x.toFixed(1)} ${p1.y.toFixed(1)}`;
  }
  return d;
}

const sparklinePoints = computed<SplinePoint[]>(() => {
  const data = overallTrend.value;
  if (data.length < 2) return [];
  const max = Math.max(...data.map((d) => d.downloads));
  if (max === 0) return [];
  return data.map((d, i) => ({
    x: (i / (data.length - 1)) * 800,
    y: 10 + (1 - d.downloads / max) * 100,
  }));
});

const sparklinePath = computed(() => buildSplinePath(sparklinePoints.value));

const sparklineAreaPath = computed(() => {
  const pts = sparklinePoints.value;
  if (pts.length < 2) return "";
  const line = sparklinePath.value;
  const last = pts[pts.length - 1];
  const first = pts[0];
  return `${line} L ${last.x.toFixed(1)} 120 L ${first.x.toFixed(1)} 120 Z`;
});

const sparklineXLabels = computed(() => {
  const data = overallTrend.value;
  if (data.length < 2) return [];
  const mid = Math.floor((data.length - 1) / 2);
  return [
    data[0]?.category ?? "",
    data[mid]?.category ?? "",
    data[data.length - 1]?.category ?? "",
  ];
});
</script>

<template>
  <div>
    <!-- Period toggle -->
    <div class="mb-6 flex flex-wrap items-center gap-1">
      <button
        v-for="opt in periodOptions"
        :key="opt.value"
        class="cursor-pointer rounded-md px-3 py-1.5 font-mono text-xs transition-colors"
        :class="
          period === opt.value
            ? 'bg-[var(--color-brand-muted)] text-[var(--color-brand)] ring-1 ring-[var(--color-brand-border)]'
            : 'text-muted hover:text-zinc-700 dark:hover:text-zinc-300'
        "
        @click="setPeriod(opt.value)"
      >
        {{ opt.label }}
      </button>
      <span v-if="dateRangeLabel" class="ml-3 font-mono text-xs text-muted">
        {{ dateRangeLabel }}
      </span>
    </div>

    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <div v-else-if="stats" class="space-y-8">
      <!-- Hero stats row -->
      <div v-if="overallTrend.length" class="mb-6 grid grid-cols-3 gap-3">
        <div class="rounded-lg border border-subtle bg-surface px-4 py-3">
          <div class="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.07em] text-muted">
            {{
              period === "4w" ? "Last 4 weeks" : period === "3m" ? "Last 3 months" : "Last 6 months"
            }}
          </div>
          <div class="text-2xl font-bold tracking-tight text-primary">
            {{ formatDownloads(totalDownloads) }}
          </div>
          <div class="mt-0.5 text-xs text-muted">downloads</div>
        </div>
        <div class="rounded-lg border border-subtle bg-surface px-4 py-3">
          <div class="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.07em] text-muted">
            Weekly average
          </div>
          <div class="text-2xl font-bold tracking-tight text-primary">
            {{ formatDownloads(weeklyAverage) }}
          </div>
          <div class="mt-0.5 text-xs text-muted">downloads / week</div>
        </div>
        <div class="rounded-lg border border-subtle bg-surface px-4 py-3">
          <div class="mb-1.5 text-[10.5px] font-medium uppercase tracking-[0.07em] text-muted">
            Peak week
          </div>
          <div class="text-2xl font-bold tracking-tight text-primary">
            {{ peakWeek ? formatDownloads(peakWeek.downloads) : "—" }}
          </div>
          <div class="mt-0.5 text-xs text-muted">{{ peakWeek?.category ?? "" }}</div>
        </div>
      </div>

      <!-- Download Trend sparkline -->
      <div v-if="sparklinePoints.length" class="mb-6">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-xs font-semibold uppercase tracking-[0.07em] text-muted">
            Download Trend
          </h2>
        </div>
        <div class="overflow-hidden rounded-lg border border-subtle bg-surface px-4 pb-2.5 pt-4">
          <svg
            viewBox="0 0 800 120"
            preserveAspectRatio="none"
            class="w-full"
            style="height: 120px; display: block"
            aria-hidden="true"
          >
            <defs>
              <linearGradient id="spark-grad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color="rgba(74,222,128,0.25)" />
                <stop offset="100%" stop-color="rgba(74,222,128,0)" />
              </linearGradient>
            </defs>
            <!-- Grid lines -->
            <line
              x1="0"
              y1="35"
              x2="800"
              y2="35"
              stroke="rgba(255,255,255,0.04)"
              stroke-width="1"
            />
            <line
              x1="0"
              y1="60"
              x2="800"
              y2="60"
              stroke="rgba(255,255,255,0.04)"
              stroke-width="1"
            />
            <line
              x1="0"
              y1="85"
              x2="800"
              y2="85"
              stroke="rgba(255,255,255,0.04)"
              stroke-width="1"
            />
            <!-- Area fill -->
            <path :d="sparklineAreaPath" fill="url(#spark-grad)" />
            <!-- Line -->
            <path
              :d="sparklinePath"
              fill="none"
              stroke="rgba(74,222,128,0.8)"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
            <!-- Dots -->
            <circle
              v-for="(pt, i) in sparklinePoints"
              :key="i"
              :cx="pt.x"
              :cy="pt.y"
              :r="i === sparklinePoints.length - 1 ? 4 : 3"
              fill="rgba(74,222,128,0.9)"
            />
          </svg>
          <!-- X-axis labels -->
          <div class="mt-1.5 flex justify-between">
            <span
              v-for="label in sparklineXLabels"
              :key="label"
              class="font-mono text-[9.5px] text-muted opacity-60"
              >{{ label }}</span
            >
          </div>
        </div>
      </div>

      <!-- Breakdown grid: Python version + OS side by side -->
      <div v-if="pythonVersions.length || systems.length" class="grid gap-6 sm:grid-cols-2">
        <!-- By Python Version -->
        <div v-if="pythonVersions.length">
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-[0.07em] text-muted">
            By Python Version
          </h2>
          <div class="space-y-2">
            <div
              v-for="point in pythonVersions"
              :key="point.category"
              class="flex items-center gap-3"
            >
              <span class="w-12 shrink-0 font-mono text-xs text-indigo-600 dark:text-indigo-400">{{
                point.category
              }}</span>
              <div class="flex-1">
                <div
                  class="h-1.5 rounded-full bg-indigo-500/50 dark:bg-indigo-500/30"
                  :style="{ width: barWidth(point.downloads, maxDownloads(pythonVersions)) }"
                />
              </div>
              <span class="w-10 shrink-0 text-right font-mono text-xs text-muted">
                {{ formatDownloads(point.downloads) }}
              </span>
            </div>
          </div>
        </div>

        <!-- By Operating System -->
        <div v-if="systems.length">
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-[0.07em] text-muted">
            By Operating System
          </h2>
          <div class="space-y-2">
            <div v-for="point in systems" :key="point.category" class="flex items-center gap-3">
              <span class="w-12 shrink-0 font-mono text-xs text-amber-700 dark:text-amber-400">{{
                point.category
              }}</span>
              <div class="flex-1">
                <div
                  class="h-1.5 rounded-full bg-amber-500/50 dark:bg-amber-400/30"
                  :style="{ width: barWidth(point.downloads, maxDownloads(systems)) }"
                />
              </div>
              <span class="w-10 shrink-0 text-right font-mono text-xs text-muted">
                {{ formatDownloads(point.downloads) }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
