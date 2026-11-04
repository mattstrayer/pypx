<script setup lang="ts">
import { formatInline } from "~/composables/useDocstringFormat";

defineProps<{
  text: string;
}>();
</script>

<template>
  <!-- formatInline escapes the text before it adds any tag. -->
  <span class="inline-markup" v-html="formatInline(text)" />
</template>

<style scoped>
.inline-markup :deep(code) {
  background: var(--color-raised);
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--py-name);
  font-size: 0.95em;
  font-family: var(--font-mono);
}
</style>
