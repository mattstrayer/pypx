<script setup lang="ts">
defineProps<{
  keys: string;
}>();
</script>

<template>
  <kbd
    class="pointer-events-none ml-1 hidden rounded bg-zinc-800 px-1 py-0.5 font-mono text-[10px] text-zinc-600 group-hover:text-zinc-400 sm:inline-block"
  >
    {{ keys }}
  </kbd>
</template>
