<script setup lang="ts">
import type { SearchResult } from "~/types/api";

const props = defineProps<{
  packages: SearchResult[];
}>();

const maxDownloads = computed(() =>
  props.packages.reduce((max, p) => Math.max(max, p.downloads), 1),
);
</script>

<template>
  <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
    <NuxtLink
      v-for="pkg in packages"
      :key="pkg.name"
      :to="`/packages/${pkg.name}`"
      class="group flex flex-col gap-1.5 rounded-[10px] border border-subtle bg-surface px-4 py-3.5 transition-colors hover:border-[rgba(74,222,128,0.3)] hover:bg-[rgba(74,222,128,0.03)]"
    >
      <div class="flex items-center justify-between gap-2">
        <h3
          class="text-[13.5px] font-semibold leading-tight tracking-[-0.01em] text-primary transition-colors group-hover:text-[var(--color-brand)]"
        >
          {{ pkg.name }}
        </h3>
        <span class="shrink-0 font-mono text-[10.5px] text-muted"
          >{{ formatDownloads(pkg.downloads) }}/mo</span
        >
      </div>
      <p class="min-h-[34px] text-[11.5px] leading-[1.5] text-muted line-clamp-2">
        {{ pkg.summary }}
      </p>
      <!-- Proportional download bar -->
      <div class="mt-0.5 h-0.5 overflow-hidden rounded-full bg-raised">
        <div
          class="h-full rounded-full bg-gradient-to-r from-[rgba(74,222,128,0.5)] to-[rgba(74,222,128,0.25)]"
          :style="{ width: `${(pkg.downloads / maxDownloads) * 100}%` }"
        />
      </div>
    </NuxtLink>
  </div>
</template>
