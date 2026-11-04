<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  member: DocSymbol;
}>();

const isClass = computed(() => props.member.kind === "class");
const shownParams = computed(() =>
  (props.member.parameters ?? []).filter((p) => p.name !== "self" && p.name !== "cls"),
);
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

    <div v-if="shownParams.length" class="mb-2">
      <p class="section-label">Parameters</p>
      <div class="space-y-1">
        <div
          v-for="param in shownParams"
          :key="param.name"
          class="flex flex-wrap items-baseline gap-2 rounded-md border-l-2 border-subtle bg-raised px-3 py-1.5"
        >
          <span class="min-w-0 font-mono text-[12px] text-sky-400 [overflow-wrap:anywhere]">{{
            param.name
          }}</span>
          <span
            v-if="param.type"
            class="min-w-0 font-mono text-[11px] text-muted [overflow-wrap:anywhere]"
            >{{ param.type }}</span
          >
          <DocsPyInlineMarkup
            v-if="param.description"
            :text="param.description"
            class="ml-1 min-w-0 text-[11.5px] text-muted [overflow-wrap:anywhere]"
          />
        </div>
      </div>
    </div>

    <div v-if="member.returns" class="mb-2">
      <p class="section-label">Returns</p>
      <div
        class="flex flex-wrap items-baseline gap-2 rounded-md border-l-2 border-[rgba(74,222,128,0.4)] bg-raised px-3 py-1.5"
      >
        <span
          v-if="member.returns.type"
          class="min-w-0 font-mono text-[12px] text-sky-400 [overflow-wrap:anywhere]"
          >{{ member.returns.type }}</span
        >
        <DocsPyInlineMarkup
          v-if="member.returns.description"
          :text="member.returns.description"
          class="min-w-0 text-[11.5px] text-muted [overflow-wrap:anywhere]"
        />
      </div>
    </div>

    <div v-if="member.raises && member.raises.length" class="mb-2">
      <p class="section-label">Raises</p>
      <div class="space-y-1">
        <div
          v-for="r in member.raises"
          :key="r.type"
          class="flex flex-wrap items-baseline gap-2 rounded-md border-l-2 border-red-500/30 bg-raised px-3 py-1.5"
        >
          <span class="min-w-0 font-mono text-[12px] text-red-400 [overflow-wrap:anywhere]">{{
            r.type
          }}</span>
          <DocsPyInlineMarkup
            v-if="r.description"
            :text="r.description"
            class="min-w-0 text-[11.5px] text-muted [overflow-wrap:anywhere]"
          />
        </div>
      </div>
    </div>

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
