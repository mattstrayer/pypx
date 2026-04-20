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

      <!-- Download Trends -->
      <div v-if="overallTrend.length">
        <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">
          Download Trends
        </h2>
        <div class="space-y-2">
          <div v-for="point in overallTrend" :key="point.category" class="flex items-center gap-3">
            <span class="w-20 shrink-0 font-mono text-xs text-muted">{{ point.category }}</span>
            <div class="flex-1">
              <div
                class="h-4 rounded-sm bg-emerald-500/60 dark:bg-emerald-400/30"
                :style="{ width: barWidth(point.downloads, maxDownloads(overallTrend)) }"
              />
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-xs text-muted">
              {{ formatDownloads(point.downloads) }}
            </span>
          </div>
        </div>
      </div>

      <!-- By Python Version -->
      <div v-if="pythonVersions.length">
        <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">
          By Python Version
        </h2>
        <div class="space-y-2">
          <div
            v-for="point in pythonVersions"
            :key="point.category"
            class="flex items-center gap-3"
          >
            <span class="w-20 shrink-0 font-mono text-xs text-indigo-600 dark:text-indigo-400">{{
              point.category
            }}</span>
            <div class="flex-1">
              <div
                class="h-4 rounded-sm bg-indigo-500/50 dark:bg-indigo-500/30"
                :style="{ width: barWidth(point.downloads, maxDownloads(pythonVersions)) }"
              />
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-xs text-muted">
              {{ formatDownloads(point.downloads) }}
            </span>
          </div>
        </div>
      </div>

      <!-- By Operating System -->
      <div v-if="systems.length">
        <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">
          By Operating System
        </h2>
        <div class="space-y-2">
          <div v-for="point in systems" :key="point.category" class="flex items-center gap-3">
            <span class="w-20 shrink-0 font-mono text-xs text-amber-700 dark:text-amber-400">{{
              point.category
            }}</span>
            <div class="flex-1">
              <div
                class="h-4 rounded-sm bg-amber-500/50 dark:bg-amber-400/30"
                :style="{ width: barWidth(point.downloads, maxDownloads(systems)) }"
              />
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-xs text-muted">
              {{ formatDownloads(point.downloads) }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
