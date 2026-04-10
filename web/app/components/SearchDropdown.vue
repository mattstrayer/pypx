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
    class="overflow-hidden rounded-lg border border-zinc-800 bg-zinc-900/95 shadow-xl backdrop-blur"
  >
    <!-- Loading -->
    <div v-if="loading && results.length === 0" class="px-3 py-4 text-center text-sm text-zinc-500">
      Searching...
    </div>

    <!-- Results -->
    <template v-else-if="results.length > 0">
      <button
        v-for="(result, index) in results"
        :key="result.name"
        class="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors"
        :class="index === selectedIndex ? 'bg-zinc-800' : 'hover:bg-zinc-800/50'"
        @click="emit('select', result)"
        @mousemove="emit('hover', index)"
      >
        <span
          class="shrink-0 text-sm font-medium"
          :class="index === selectedIndex ? 'text-zinc-50' : 'text-zinc-200'"
        >
          {{ result.name }}
        </span>
        <span
          v-if="result.summary"
          class="truncate text-sm"
          :class="index === selectedIndex ? 'text-zinc-400' : 'text-zinc-500'"
        >
          {{ result.summary }}
        </span>
      </button>

      <!-- Footer -->
      <div
        class="flex items-center justify-between border-t border-zinc-800 px-3 py-1.5 text-xs text-zinc-600"
      >
        <span>↑↓ navigate · ↵ select · esc close</span>
        <span>{{ results.length }} results</span>
      </div>
    </template>

    <!-- No results -->
    <div v-else class="px-3 py-4 text-center text-sm text-zinc-500">No packages found</div>
  </div>
</template>
