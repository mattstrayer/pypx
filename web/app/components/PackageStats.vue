<script setup lang="ts">
import type { DataPoint } from "~/types/api";

const props = defineProps<{
  name: string;
}>();

const { fetchStats } = useApi();
const { data: stats, status } = await useAsyncData(`stats-${props.name}`, () =>
  fetchStats(props.name),
);

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
</script>

<template>
  <div>
    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>

    <div v-else-if="stats" class="space-y-8">
      <!-- Download Trends -->
      <div v-if="overallTrend.length">
        <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-zinc-500">
          Download Trends
        </h2>
        <div class="space-y-2">
          <div v-for="point in overallTrend" :key="point.category" class="flex items-center gap-3">
            <span class="w-20 shrink-0 font-mono text-xs text-zinc-500">{{ point.category }}</span>
            <div class="flex-1">
              <div
                class="h-4 rounded-sm bg-emerald-500/30"
                :style="{ width: barWidth(point.downloads, maxDownloads(overallTrend)) }"
              />
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-xs text-zinc-400">
              {{ formatDownloads(point.downloads) }}
            </span>
          </div>
        </div>
      </div>

      <!-- By Python Version -->
      <div v-if="pythonVersions.length">
        <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-zinc-500">
          By Python Version
        </h2>
        <div class="space-y-2">
          <div
            v-for="point in pythonVersions"
            :key="point.category"
            class="flex items-center gap-3"
          >
            <span class="w-20 shrink-0 font-mono text-xs text-indigo-400">{{
              point.category
            }}</span>
            <div class="flex-1">
              <div
                class="h-4 rounded-sm bg-indigo-500/30"
                :style="{ width: barWidth(point.downloads, maxDownloads(pythonVersions)) }"
              />
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-xs text-zinc-400">
              {{ formatDownloads(point.downloads) }}
            </span>
          </div>
        </div>
      </div>

      <!-- By Operating System -->
      <div v-if="systems.length">
        <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-zinc-500">
          By Operating System
        </h2>
        <div class="space-y-2">
          <div v-for="point in systems" :key="point.category" class="flex items-center gap-3">
            <span class="w-20 shrink-0 font-mono text-xs text-amber-400">{{ point.category }}</span>
            <div class="flex-1">
              <div
                class="h-4 rounded-sm bg-amber-500/30"
                :style="{ width: barWidth(point.downloads, maxDownloads(systems)) }"
              />
            </div>
            <span class="w-12 shrink-0 text-right font-mono text-xs text-zinc-400">
              {{ formatDownloads(point.downloads) }}
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
