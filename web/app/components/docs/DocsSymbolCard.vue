<script setup lang="ts">
import type { DocEntry } from "~/utils/docSymbols";

const props = defineProps<{
  symbol: DocEntry;
  isActive: boolean;
}>();

const hasMembers = computed(
  () => props.symbol.kind === "class" || props.symbol.kind === "exception",
);

const MAX_SHOWN_ALIASES = 5;
const allAliasesShown = ref(false);
const hiddenAliasCount = computed(() => props.symbol.importableAs.length - MAX_SHOWN_ALIASES);
const shownAliases = computed(() =>
  allAliasesShown.value
    ? props.symbol.importableAs
    : props.symbol.importableAs.slice(0, MAX_SHOWN_ALIASES),
);
</script>

<template>
  <div
    :id="`sym-${encodeURIComponent(symbol.id)}`"
    class="mb-4 scroll-mt-16 rounded-xl border bg-surface p-5 transition-[border-color,background-color]"
    :class="
      isActive
        ? 'border-[var(--color-brand-border)] bg-[rgba(74,222,128,0.03)] shadow-[0_0_0_1px_rgba(74,222,128,0.08)]'
        : 'border-subtle hover:border-zinc-300 dark:hover:border-zinc-600'
    "
  >
    <!-- Symbol name + kind badge -->
    <p data-testid="symbol-module" class="mb-1 truncate font-mono text-[11px] text-muted">
      {{ symbol.module }}
    </p>
    <p
      v-if="symbol.origin"
      data-testid="symbol-origin"
      class="mb-1 truncate text-[11px] text-muted"
    >
      Defined in <span class="font-mono">{{ symbol.origin }}</span>
    </p>
    <div class="mb-3 flex flex-wrap items-center gap-2.5">
      <span class="font-mono text-[15px] font-bold text-primary">{{ symbol.name }}</span>
      <span
        class="rounded border px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em]"
        :class="{
          'border-blue-500/20 bg-blue-950/60 text-blue-300': symbol.kind === 'function',
          'border-purple-500/20 bg-purple-950/60 text-purple-300': symbol.kind === 'class',
          'border-red-500/20 bg-red-950/60 text-red-300': symbol.kind === 'exception',
        }"
        >{{ symbol.kind }}</span
      >
      <DocsBadges :symbol="symbol" />
    </div>

    <!-- Other names for this symbol -->
    <p
      v-if="symbol.importableAs.length"
      data-testid="symbol-aliases"
      class="mb-3 text-[11px] text-muted [overflow-wrap:anywhere]"
    >
      Also importable as
      <template v-for="(path, i) in shownAliases" :key="path">
        <span class="font-mono">{{ path }}</span
        >{{ i < shownAliases.length - 1 ? ", " : "" }}
      </template>
      <button
        v-if="hiddenAliasCount > 0"
        data-testid="symbol-aliases-toggle"
        class="ml-1 cursor-pointer text-[var(--color-brand)] hover:underline"
        :aria-expanded="allAliasesShown"
        @click="allAliasesShown = !allAliasesShown"
      >
        {{ allAliasesShown ? "show fewer" : `+${hiddenAliasCount} more` }}
      </button>
    </p>

    <!-- Signature -->
    <DocsPySignature :symbol="symbol" class="mb-3" />
    <DocsOverloads v-if="symbol.overloads?.length" :overloads="symbol.overloads" class="mb-3" />

    <!-- Docstring -->
    <DocsPyDocstring v-if="symbol.docstring" :text="symbol.docstring" class="mb-4" />

    <DocsSymbolFields :symbol="symbol" />

    <!-- Attributes -->
    <div v-if="hasMembers && symbol.attributes && symbol.attributes.length" class="mb-4">
      <p class="section-label">Attributes</p>
      <div class="space-y-1.5">
        <DocsAttributeRow v-for="attr in symbol.attributes" :key="attr.name" :attribute="attr" />
      </div>
    </div>

    <!-- Methods (classes and exceptions) -->
    <DocsMemberFold
      v-if="hasMembers && symbol.methods && symbol.methods.length"
      title="Methods"
      :members="symbol.methods"
    />

    <!-- Nested classes -->
    <DocsMemberFold
      v-if="hasMembers && symbol.classes && symbol.classes.length"
      title="Nested classes"
      :members="symbol.classes"
    />
  </div>
</template>
