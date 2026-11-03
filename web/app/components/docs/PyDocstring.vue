<script setup lang="ts">
import { formatDocstring, renderDocstring } from "~/composables/useDocstringFormat";
import { highlightPython } from "~/utils/shikiHighlight";

const props = defineProps<{
  text: string;
}>();

const formattedHtml = computedAsync(async () => {
  if (!props.text?.trim()) return "";

  // During SSR, highlight code blocks with Shiki.
  return import.meta.server
    ? renderDocstring(props.text, highlightPython)
    : formatDocstring(props.text);
}, "");
</script>

<template>
  <div
    v-if="formattedHtml"
    class="docstring-content text-sm leading-relaxed text-muted"
    v-html="formattedHtml"
  />
</template>

<style scoped>
.docstring-content :deep(p) {
  margin-bottom: 0.75rem;
}
.docstring-content :deep(code) {
  background: var(--color-raised);
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--py-name);
  font-size: 0.85em;
  font-family: var(--font-mono);
}
.docstring-content :deep(pre) {
  background: var(--color-base);
  border: 1px solid var(--color-subtle);
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 0.75rem;
  overflow-x: auto;
}
.docstring-content :deep(pre code) {
  background: transparent;
  padding: 0;
  border-radius: 0;
  color: inherit;
  font-size: 11px;
}
</style>
