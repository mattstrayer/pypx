<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  symbol: Pick<DocSymbol, "parameters" | "returns" | "raises">;
  /** A method or nested class inside a fold: closer rows that always wrap. */
  dense?: boolean;
}>();

const shownParams = computed(() =>
  (props.symbol.parameters ?? []).filter((p) => p.name !== "self" && p.name !== "cls"),
);

// A card keeps its label when self or cls is the only parameter, so this component does
// not move a card at desktop width. Removing that empty label is a separate change.
const showParams = computed(() =>
  props.dense ? shownParams.value.length > 0 : Boolean(props.symbol.parameters?.length),
);

const sectionClass = computed(() => (props.dense ? "mb-2" : "mb-4"));
const listClass = computed(() => (props.dense ? "space-y-1" : "space-y-1.5"));

// One row for all three sections. Below lg a row wraps, so a long type or description
// takes its own line. A card has the width to keep a row on one line from lg up.
const rowClass = computed(() => [
  "flex flex-wrap items-baseline gap-2 rounded-md border-l-2 bg-raised px-3 py-1.5",
  props.dense ? "[overflow-wrap:anywhere]" : "lg:flex-nowrap",
]);
</script>

<template>
  <div v-if="showParams" :class="sectionClass">
    <p class="section-label">Parameters</p>
    <div :class="listClass">
      <div
        v-for="param in shownParams"
        :key="param.name"
        data-testid="field-row"
        :class="[rowClass, 'border-subtle']"
      >
        <span class="font-mono text-[12px] text-sky-400">{{ param.name }}</span>
        <span v-if="param.type" class="font-mono text-[11px] text-muted">{{ param.type }}</span>
        <DocsPyInlineMarkup
          v-if="param.description"
          :text="param.description"
          class="ml-1 text-[11.5px] text-muted"
        />
      </div>
    </div>
  </div>

  <div v-if="symbol.returns" :class="sectionClass">
    <p class="section-label">Returns</p>
    <div data-testid="field-row" :class="[rowClass, 'border-[rgba(74,222,128,0.4)]']">
      <span v-if="symbol.returns.type" class="font-mono text-[12px] text-sky-400">{{
        symbol.returns.type
      }}</span>
      <DocsPyInlineMarkup
        v-if="symbol.returns.description"
        :text="symbol.returns.description"
        class="text-[11.5px] text-muted"
      />
    </div>
  </div>

  <div v-if="symbol.raises?.length" :class="sectionClass">
    <p class="section-label">Raises</p>
    <div :class="listClass">
      <div
        v-for="r in symbol.raises"
        :key="r.type"
        data-testid="field-row"
        :class="[rowClass, 'border-red-500/30']"
      >
        <span class="font-mono text-[12px] text-red-400">{{ r.type }}</span>
        <DocsPyInlineMarkup
          v-if="r.description"
          :text="r.description"
          class="text-[11.5px] text-muted"
        />
      </div>
    </div>
  </div>
</template>
