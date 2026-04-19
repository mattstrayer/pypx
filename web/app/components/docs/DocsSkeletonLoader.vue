<script setup lang="ts">
defineProps<{
  mode: "fetch" | "render";
  renderedCount?: number;
  totalCount?: number;
}>();
</script>

<template>
  <!-- Full fetch skeleton: matches two-column layout to prevent layout shift -->
  <div v-if="mode === 'fetch'" class="flex gap-0 -mx-4 sm:-mx-6 lg:-mx-8">
    <!-- Sidebar skeleton -->
    <div
      class="w-48 flex-shrink-0 sticky top-0 h-screen border-r border-subtle bg-base py-3 hidden md:block"
    >
      <div class="px-3 pb-3">
        <div class="mb-3 h-2 w-16 rounded bg-zinc-800 animate-pulse" />
        <div class="space-y-2">
          <div
            v-for="i in 8"
            :key="i"
            class="h-5 rounded bg-zinc-800 animate-pulse"
            :style="`width: ${55 + (i % 3) * 15}%`"
          />
        </div>
      </div>
    </div>
    <!-- Content skeleton -->
    <div class="flex-1 min-w-0 px-6 py-5 space-y-10">
      <div v-for="i in 3" :key="i" class="space-y-3">
        <div class="flex items-center gap-3">
          <div class="h-5 w-32 rounded bg-zinc-800 animate-pulse" />
          <div class="h-4 w-16 rounded bg-zinc-800 animate-pulse" />
        </div>
        <div class="h-8 w-full rounded bg-zinc-800 animate-pulse" />
        <div class="space-y-2">
          <div class="h-3 w-full rounded bg-zinc-800 animate-pulse" />
          <div class="h-3 w-5/6 rounded bg-zinc-800 animate-pulse" />
          <div class="h-3 w-4/6 rounded bg-zinc-800 animate-pulse" />
        </div>
      </div>
    </div>
  </div>

  <!-- Render-progress indicator: shown below already-rendered content -->
  <div v-else-if="mode === 'render'" class="py-4 text-center text-xs text-zinc-600">
    Loading {{ renderedCount }} of {{ totalCount }} symbols...
  </div>
</template>
