<script setup lang="ts">
import { formatDocstring, renderDocstring } from "~/composables/useDocstringFormat";

const props = defineProps<{
  text: string;
}>();

// Load Shiki only when a docstring holds code, to keep it out of the initial bundle.
async function highlightLazily(code: string): Promise<string> {
  const { highlightPython } = await import("~/utils/shikiHighlight");
  return highlightPython(code);
}

const highlighted = computedAsync(async () => {
  const text = props.text;
  if (import.meta.server) return null;
  try {
    return { text, html: await renderDocstring(text, highlightLazily) };
  } catch {
    // The plain block stays on the page, so a failed chunk load costs only colour.
    return null;
  }
}, null);

const formattedHtml = computed(() =>
  highlighted.value?.text === props.text ? highlighted.value.html : formatDocstring(props.text),
);
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
