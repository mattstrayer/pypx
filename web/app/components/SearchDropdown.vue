<script setup lang="ts">
import type { SearchResult } from "~/types/api";

defineProps<{
  results: SearchResult[];
  selectedIndex: number;
  loading: boolean;
  hasQuery: boolean;
}>();

const emit = defineEmits<{
  select: [result: SearchResult];
  hover: [index: number];
}>();
</script>

<template>
  <div
    v-if="hasQuery"
    role="listbox"
    aria-label="Search results"
    class="overflow-hidden rounded-lg border border-subtle bg-surface/95 shadow-xl backdrop-blur"
  >
    <!-- Loading -->
    <div v-if="loading && results.length === 0" class="px-3 py-4 text-center text-sm text-muted">
      Searching...
    </div>

    <!-- Results -->
    <template v-else-if="results.length > 0">
      <button
        v-for="(result, index) in results"
        :key="result.name"
        role="option"
        :aria-selected="index === selectedIndex"
        class="cursor-pointer flex w-full items-center gap-2 px-3 py-2 text-left transition-colors"
        :class="index === selectedIndex ? 'bg-raised' : 'hover:bg-raised/50'"
        @click="emit('select', result)"
        @mousemove="emit('hover', index)"
      >
        <span
          class="shrink-0 text-sm font-medium"
          :class="index === selectedIndex ? 'text-primary' : 'text-zinc-800 dark:text-zinc-200'"
        >
          {{ result.name }}
        </span>
        <span
          v-if="result.summary"
          class="truncate text-sm"
          :class="index === selectedIndex ? 'text-muted' : 'text-muted'"
        >
          {{ result.summary }}
        </span>
      </button>

      <!-- Footer -->
      <div
        class="flex items-center justify-between border-t border-subtle px-3 py-1.5 text-xs text-zinc-400 dark:text-zinc-600"
      >
        <span>↑↓ navigate · ↵ select · esc close</span>
        <span>{{ results.length }} results</span>
      </div>
    </template>

    <!-- No results -->
    <div v-else class="px-3 py-4 text-center text-sm text-muted">No packages found</div>
  </div>
</template>
