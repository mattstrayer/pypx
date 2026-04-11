<script setup lang="ts">
import type { SearchResult } from "~/types/api";

defineProps<{
  results: SearchResult[];
}>();

function formatDownloads(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}
</script>

<template>
  <div class="space-y-2">
    <NuxtLink
      v-for="pkg in results"
      :key="pkg.name"
      :to="`/packages/${pkg.name}`"
      class="flex items-start justify-between rounded-lg border border-zinc-800 bg-zinc-900/50 p-4 transition-colors hover:border-zinc-700 hover:bg-zinc-900"
    >
      <div class="min-w-0 flex-1 pr-4">
        <span class="font-mono font-semibold text-zinc-50 hover:text-[var(--color-brand)]">{{
          pkg.name
        }}</span>
        <p v-if="pkg.summary" class="mt-1 text-sm text-zinc-400 line-clamp-2">{{ pkg.summary }}</p>
      </div>
      <span class="shrink-0 font-mono text-xs text-zinc-500"
        >{{ formatDownloads(pkg.downloads) }}/mo</span
      >
    </NuxtLink>
  </div>
</template>
