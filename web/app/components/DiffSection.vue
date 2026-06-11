<script setup lang="ts">
defineProps<{
  title: string;
  unavailable?: string;
  empty?: boolean;
  emptyMessage?: string;
}>();
</script>

<template>
  <div class="overflow-hidden rounded-[14px] border border-subtle bg-surface">
    <div class="flex items-center gap-2 border-b border-subtle bg-raised/40 px-4 py-2.5">
      <h2 class="text-xs font-semibold uppercase tracking-[0.07em] text-muted">{{ title }}</h2>
    </div>

    <div v-if="unavailable" class="px-4 py-3 text-xs text-muted">
      Unavailable: {{ unavailable }}
    </div>
    <div v-else-if="empty" class="px-4 py-3 text-xs text-muted">
      {{ emptyMessage ?? "No data." }}
    </div>
    <slot v-else />
  </div>
</template>
