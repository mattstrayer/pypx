<script setup lang="ts">
defineProps<{
  keys: string;
}>();
</script>

<template>
  <kbd
    class="pointer-events-none ml-1 hidden rounded bg-raised px-1 py-0.5 font-mono text-[10px] text-muted sm:inline-block"
  >
    {{ keys }}
  </kbd>
</template>
