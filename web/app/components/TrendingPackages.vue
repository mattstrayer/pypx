<script setup lang="ts">
import type { SearchResult } from "~/types/api";

defineProps<{
  packages: SearchResult[];
}>();

function formatDownloads(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}
</script>

<template>
  <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
    <NuxtLink
      v-for="pkg in packages"
      :key="pkg.name"
      :to="`/packages/${pkg.name}`"
      class="group rounded-lg border border-zinc-800 bg-zinc-900/50 p-4 transition-colors hover:border-zinc-700 hover:bg-zinc-900"
    >
      <div class="flex items-start justify-between">
        <h3 class="font-semibold text-zinc-50 group-hover:text-white">{{ pkg.name }}</h3>
        <span class="font-mono text-xs text-zinc-500">{{ formatDownloads(pkg.downloads) }}/mo</span>
      </div>
      <p class="mt-1 text-sm text-zinc-400 line-clamp-2">{{ pkg.summary }}</p>
    </NuxtLink>
  </div>
</template>
