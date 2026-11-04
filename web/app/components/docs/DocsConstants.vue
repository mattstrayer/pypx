<script setup lang="ts">
import type { DocConstantEntry } from "~/utils/docSymbols";

const props = defineProps<{
  constants: DocConstantEntry[];
  activeSymbol: string | null;
}>();

// The list is in module order, so one module's constants are next to each other.
const groups = computed(() => {
  const out: { module: string; constants: DocConstantEntry[] }[] = [];
  for (const c of props.constants) {
    const last = out[out.length - 1];
    if (last?.module === c.module) last.constants.push(c);
    else out.push({ module: c.module, constants: [c] });
  }
  return out;
});
</script>

<template>
  <section id="constants" data-testid="constants" class="mt-8 scroll-mt-16">
    <h2 class="mb-3 text-sm font-semibold text-primary">Constants</h2>
    <div
      v-for="group in groups"
      :key="group.module"
      class="mb-4 rounded-xl border border-subtle bg-surface p-5"
    >
      <h3 class="mb-2 font-mono text-[11px] font-normal text-muted [overflow-wrap:anywhere]">
        {{ group.module }}
      </h3>
      <div class="space-y-1.5">
        <DocsAttributeRow
          v-for="c in group.constants"
          :id="`sym-${encodeURIComponent(c.id)}`"
          :key="c.id"
          :attribute="c"
          :data-symbol="c.id"
          data-testid="constant-row"
          class="scroll-mt-16"
          :class="activeSymbol === c.id ? 'border-[var(--color-brand)]!' : ''"
        />
      </div>
    </div>
  </section>
</template>
