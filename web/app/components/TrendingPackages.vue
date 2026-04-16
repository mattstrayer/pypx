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
      class="group rounded-lg border border-subtle bg-surface p-4 transition-colors hover:border-zinc-300 dark:hover:border-zinc-700 hover:bg-surface"
    >
      <div class="flex items-start justify-between">
        <h3
          class="font-semibold text-primary group-hover:text-[var(--color-brand)] transition-colors"
        >
          {{ pkg.name }}
        </h3>
        <span class="font-mono text-xs text-muted">{{ formatDownloads(pkg.downloads) }}/mo</span>
      </div>
      <p class="mt-1 text-sm text-muted line-clamp-2">{{ pkg.summary }}</p>
    </NuxtLink>
  </div>
</template>
