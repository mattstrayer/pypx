<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  member: DocSymbol;
}>();

const isClass = computed(() => props.member.kind === "class");
</script>

<template>
  <div data-testid="member-entry">
    <div class="mb-2 flex flex-wrap items-center gap-2">
      <span class="font-mono text-[12px] font-semibold text-primary">{{ member.name }}</span>
      <span
        v-if="isClass"
        class="rounded border border-purple-500/20 bg-purple-950/60 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-purple-300"
        >class</span
      >
      <span
        v-else
        class="rounded border border-blue-500/20 bg-blue-950/60 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-blue-300"
        >method</span
      >
      <DocsBadges :symbol="member" />
    </div>

    <DocsPySignature :symbol="member" class="mb-2" />
    <DocsOverloads v-if="member.overloads?.length" :overloads="member.overloads" class="mb-2" />
    <DocsPyDocstring v-if="member.docstring" :text="member.docstring" class="mb-2" />

    <DocsSymbolFields :symbol="member" dense />

    <template v-if="isClass">
      <div v-if="member.attributes && member.attributes.length" class="mb-2">
        <p class="section-label">Attributes</p>
        <div class="space-y-1">
          <DocsAttributeRow v-for="attr in member.attributes" :key="attr.name" :attribute="attr" />
        </div>
      </div>
      <DocsMemberFold v-if="member.methods?.length" title="Methods" :members="member.methods" />
      <DocsMemberFold
        v-if="member.classes?.length"
        title="Nested classes"
        :members="member.classes"
      />
    </template>
  </div>
</template>
