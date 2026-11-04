<script setup lang="ts">
import type { DocSymbol } from "~/types/api";
import { docBadges } from "~/utils/docSymbols";

const props = defineProps<{
  symbol: Pick<DocSymbol, "method_kind" | "decorators">;
}>();

const badges = computed(() => docBadges(props.symbol));
</script>

<template>
  <span
    v-for="badge in badges"
    :key="badge"
    data-testid="symbol-badge"
    class="rounded border border-subtle px-1 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-muted"
    >{{ badge }}</span
  >
</template>
