<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  symbol: DocSymbol;
  isActive: boolean;
}>();

const expanded = ref(false);

function toggleExpand() {
  expanded.value = !expanded.value;
}
</script>

<template>
  <div
    :id="`sym-${encodeURIComponent(symbol.name)}`"
    class="mb-4 scroll-mt-4 rounded-xl border bg-surface p-5 transition-[border-color,background-color]"
    :class="
      isActive
        ? 'border-[var(--color-brand-border)] bg-[rgba(74,222,128,0.03)] shadow-[0_0_0_1px_rgba(74,222,128,0.08)]'
        : 'border-subtle hover:border-zinc-300 dark:hover:border-zinc-600'
    "
  >
    <!-- Symbol name + kind badge -->
    <div class="mb-3 flex items-center gap-2.5">
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
    </div>

    <!-- Signature -->
    <DocsPySignature :symbol="symbol" class="mb-3" />

    <!-- Docstring -->
    <DocsPyDocstring v-if="symbol.docstring" :text="symbol.docstring" class="mb-4" />

    <!-- Parameters -->
    <div v-if="symbol.parameters && symbol.parameters.length" class="mb-4">
      <p class="section-label">Parameters</p>
      <div class="space-y-1.5">
        <div
          v-for="param in symbol.parameters?.filter((p) => p.name !== 'self' && p.name !== 'cls')"
          :key="param.name"
          class="flex items-baseline gap-2 rounded-md border-l-2 border-subtle bg-raised px-3 py-1.5"
        >
          <span class="font-mono text-[12px] text-sky-400">{{ param.name }}</span>
          <span v-if="param.type" class="font-mono text-[11px] text-muted">{{ param.type }}</span>
          <span v-if="param.description" class="ml-1 text-[11.5px] text-muted">{{
            param.description
          }}</span>
        </div>
      </div>
    </div>

    <!-- Returns -->
    <div v-if="symbol.returns" class="mb-4">
      <p class="section-label">Returns</p>
      <div
        class="flex items-baseline gap-2 rounded-md border-l-2 border-[rgba(74,222,128,0.4)] bg-raised px-3 py-1.5"
      >
        <span v-if="symbol.returns.type" class="font-mono text-[12px] text-sky-400">{{
          symbol.returns.type
        }}</span>
        <span v-if="symbol.returns.description" class="text-[11.5px] text-muted">{{
          symbol.returns.description
        }}</span>
      </div>
    </div>

    <!-- Raises -->
    <div v-if="symbol.raises && symbol.raises.length" class="mb-4">
      <p class="section-label">Raises</p>
      <div class="space-y-1.5">
        <div
          v-for="r in symbol.raises"
          :key="r.type"
          class="flex items-baseline gap-2 rounded-md border-l-2 border-red-500/30 bg-raised px-3 py-1.5"
        >
          <span class="font-mono text-[12px] text-red-400">{{ r.type }}</span>
          <span v-if="r.description" class="text-[11.5px] text-muted">{{ r.description }}</span>
        </div>
      </div>
    </div>

    <!-- Methods (classes only) -->
    <div v-if="symbol.kind === 'class' && symbol.methods && symbol.methods.length" class="mb-1">
      <button
        class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-[0.07em] text-muted transition-colors hover:bg-raised hover:text-primary"
        @click="toggleExpand"
      >
        <svg
          class="h-3 w-3 text-[var(--color-brand)] transition-transform duration-150"
          :class="expanded ? 'rotate-0' : '-rotate-90'"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
        Methods
        <span class="rounded-full bg-raised px-2 py-0.5 text-[9px] text-muted ring-1 ring-subtle">
          {{ symbol.methods.length }}
        </span>
      </button>

      <div v-if="expanded" class="mt-3 space-y-5 border-l-2 border-subtle pl-4">
        <div v-for="method in symbol.methods" :key="method.name">
          <div class="mb-2 flex items-center gap-2">
            <span class="font-mono text-[12px] font-semibold text-primary">{{ method.name }}</span>
            <span
              class="rounded border border-blue-500/20 bg-blue-950/60 px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.08em] text-blue-300"
              >method</span
            >
          </div>

          <DocsPySignature :symbol="method" class="mb-2" />
          <DocsPyDocstring v-if="method.docstring" :text="method.docstring" class="mb-2" />

          <div
            v-if="
              method.parameters &&
              method.parameters.filter((p) => p.name !== 'self' && p.name !== 'cls').length
            "
            class="mb-2"
          >
            <p class="section-label">Parameters</p>
            <div class="space-y-1">
              <div
                v-for="param in method.parameters.filter(
                  (p) => p.name !== 'self' && p.name !== 'cls',
                )"
                :key="param.name"
                class="flex items-baseline gap-2 rounded-md border-l-2 border-subtle bg-raised px-3 py-1.5"
              >
                <span class="font-mono text-[12px] text-sky-400">{{ param.name }}</span>
                <span v-if="param.type" class="font-mono text-[11px] text-muted">{{
                  param.type
                }}</span>
                <span v-if="param.description" class="ml-1 text-[11.5px] text-muted">{{
                  param.description
                }}</span>
              </div>
            </div>
          </div>

          <div v-if="method.returns" class="mb-2">
            <p class="section-label">Returns</p>
            <div
              class="flex items-baseline gap-2 rounded-md border-l-2 border-[rgba(74,222,128,0.4)] bg-raised px-3 py-1.5"
            >
              <span v-if="method.returns.type" class="font-mono text-[12px] text-sky-400">{{
                method.returns.type
              }}</span>
              <span v-if="method.returns.description" class="text-[11.5px] text-muted">{{
                method.returns.description
              }}</span>
            </div>
          </div>

          <div v-if="method.raises && method.raises.length" class="mb-2">
            <p class="section-label">Raises</p>
            <div class="space-y-1">
              <div
                v-for="r in method.raises"
                :key="r.type"
                class="flex items-baseline gap-2 rounded-md border-l-2 border-red-500/30 bg-raised px-3 py-1.5"
              >
                <span class="font-mono text-[12px] text-red-400">{{ r.type }}</span>
                <span v-if="r.description" class="text-[11.5px] text-muted">{{
                  r.description
                }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
