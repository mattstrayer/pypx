<script setup lang="ts">
defineProps<{
  status: number;
  name: string;
}>();
</script>

<template>
  <div class="py-24 text-center">
    <template v-if="status === 404">
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Package not found</p>
      <p class="mt-1 text-sm text-muted">No package named "{{ name }}" could be found.</p>
    </template>
    <template v-else>
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Package unavailable</p>
      <p class="mt-1 text-sm text-muted">
        We could not load "{{ name }}" right now. Please try again soon.
      </p>
    </template>
  </div>
</template>
