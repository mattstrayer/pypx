<script setup lang="ts">
import type { DocAttribute } from "~/types/api";

defineProps<{
  /** A constant has only some of the fields of a class attribute. */
  attribute: Pick<DocAttribute, "name"> & Partial<DocAttribute>;
}>();
</script>

<template>
  <div
    class="flex flex-wrap items-baseline gap-2 rounded-md border-l-2 border-subtle bg-raised px-3 py-1.5"
  >
    <span class="min-w-0 font-mono text-[12px] text-sky-400 [overflow-wrap:anywhere]">{{
      attribute.name
    }}</span>
    <span
      v-if="attribute.type"
      class="min-w-0 font-mono text-[11px] text-muted [overflow-wrap:anywhere]"
      >{{ attribute.type }}</span
    >
    <span
      v-if="attribute.default !== undefined"
      class="min-w-0 break-all font-mono text-[11px] text-[var(--py-default)]"
      >= {{ attribute.default }}</span
    >
    <span
      v-if="attribute.is_property"
      class="rounded border border-subtle px-1 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-muted"
      >property</span
    >
    <span
      v-if="attribute.is_property && !attribute.writable"
      class="rounded border border-subtle px-1 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-muted"
      >read-only</span
    >
    <DocsPyDocstring
      v-if="attribute.description"
      :text="attribute.description"
      class="basis-full text-[11.5px]! [&_p:last-child]:mb-0!"
    />
  </div>
</template>
