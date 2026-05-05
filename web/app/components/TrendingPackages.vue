<script setup lang="ts">
import type { SearchResult } from "~/types/api";

const props = defineProps<{
  packages: SearchResult[];
}>();

const maxDownloads = computed(() =>
  props.packages.reduce((max, p) => Math.max(max, p.downloads), 1),
);
const minDownloads = computed(() =>
  props.packages.reduce((min, p) => Math.min(min, p.downloads), maxDownloads.value),
);

// Log scale so the long tail isn't a sliver next to #1.
function logRatio(n: number): number {
  const lo = Math.log(Math.max(1, minDownloads.value * 0.6));
  const hi = Math.log(maxDownloads.value);
  if (hi <= lo) return 1;
  return Math.max(0.08, Math.min(1, (Math.log(n) - lo) / (hi - lo)));
}

const BAR_CELLS = 14;
function barFill(n: number): number {
  return Math.max(1, Math.round(logRatio(n) * BAR_CELLS));
}
</script>

<template>
  <div class="overflow-hidden rounded-[14px] border border-subtle bg-surface font-mono">
    <!-- Column header strip -->
    <div
      class="grid grid-cols-[2.5rem_1fr_auto_auto] items-center gap-3 border-b border-subtle bg-raised/40 px-4 py-2 text-[10px] uppercase tracking-[0.1em] text-muted"
    >
      <span>#</span>
      <span>Package</span>
      <span class="hidden text-right sm:block">Share</span>
      <span class="text-right">Downloads</span>
    </div>

    <ul class="divide-y divide-subtle">
      <li v-for="(pkg, i) in packages" :key="pkg.name">
        <NuxtLink
          :to="`/packages/${pkg.name}`"
          class="group grid grid-cols-[2.5rem_1fr_auto_auto] items-center gap-3 px-4 py-2.5 transition-colors hover:bg-[var(--color-brand-muted)]"
        >
          <!-- Rank -->
          <span
            class="self-start text-[11px] tabular-nums text-muted transition-colors group-hover:text-[var(--color-brand)]"
          >
            {{ String(i + 1).padStart(2, "0") }}.
          </span>

          <!-- Name (with trailing dotted leader) + summary on the next line -->
          <span class="flex min-w-0 flex-col gap-0.5 overflow-hidden">
            <span class="flex items-baseline gap-3 overflow-hidden">
              <span
                class="max-w-[16rem] shrink-0 truncate font-sans text-[13.5px] font-semibold tracking-[-0.01em] text-primary transition-colors group-hover:text-[var(--color-brand)]"
              >
                {{ pkg.name }}
              </span>
              <span
                aria-hidden="true"
                class="mb-[5px] hidden flex-1 self-end border-b border-dotted border-subtle opacity-70 sm:block"
              />
            </span>
            <span class="truncate font-sans text-[11.5px] leading-snug text-muted">
              {{ pkg.summary || "—" }}
            </span>
          </span>

          <!-- ASCII bar -->
          <span
            aria-hidden="true"
            class="hidden text-right text-[11px] leading-none tracking-tighter sm:block"
          >
            <span
              v-for="n in barFill(pkg.downloads)"
              :key="`f${n}`"
              class="text-[var(--color-brand)] opacity-80 group-hover:opacity-100"
              >▌</span
            ><span
              v-for="n in BAR_CELLS - barFill(pkg.downloads)"
              :key="`e${n}`"
              class="text-muted opacity-25"
              >▌</span
            >
          </span>

          <!-- Count -->
          <span
            class="text-right text-[11px] tabular-nums text-muted transition-colors group-hover:text-[var(--color-brand)]"
          >
            {{ formatDownloads(pkg.downloads) }}/mo
          </span>
        </NuxtLink>
      </li>
    </ul>
  </div>
</template>
