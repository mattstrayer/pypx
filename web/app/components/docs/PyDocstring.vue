<script setup lang="ts">
import { formatDocstring } from "~/composables/useDocstringFormat";
import { highlightPython } from "~/utils/shikiHighlight";

const props = defineProps<{
  text: string;
}>();

const formattedHtml = computedAsync(async () => {
  if (!props.text?.trim()) return "";

  let html = formatDocstring(props.text);

  // During SSR, replace code blocks with Shiki-highlighted versions.
  if (import.meta.server) {
    const codeBlockRegex = /<pre><code class="language-python">([\s\S]*?)<\/code><\/pre>/g;
    const matches = [...html.matchAll(codeBlockRegex)];
    for (const match of matches) {
      const code = match[1]
        .replace(/&amp;/g, "&")
        .replace(/&lt;/g, "<")
        .replace(/&gt;/g, ">")
        .replace(/&quot;/g, '"');
      const highlighted = await highlightPython(code);
      html = html.replace(match[0], highlighted);
    }
  }

  return html;
}, "");
</script>

<template>
  <div
    v-if="formattedHtml"
    class="docstring-content text-sm leading-relaxed text-zinc-400"
    v-html="formattedHtml"
  />
</template>

<style scoped>
.docstring-content :deep(p) {
  margin-bottom: 0.75rem;
}
.docstring-content :deep(code) {
  background: #27272a;
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--py-name);
  font-size: 0.85em;
  font-family: var(--font-mono);
}
.docstring-content :deep(pre) {
  background: #0f0f10;
  border: 1px solid #27272a;
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
