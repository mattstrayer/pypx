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
  <div :id="`sym-${encodeURIComponent(symbol.name)}`" class="mb-10 scroll-mt-4">
    <!-- Symbol name + kind badge -->
    <div class="mb-3 flex items-center gap-2">
      <span class="font-mono text-base font-bold text-primary">{{ symbol.name }}</span>
      <span
        class="rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
        :class="{
          'bg-blue-950 text-blue-300': symbol.kind === 'function',
          'bg-purple-950 text-purple-300': symbol.kind === 'class',
          'bg-red-950 text-red-300': symbol.kind === 'exception',
        }"
        >{{ symbol.kind }}</span
      >
    </div>

    <!-- Signature -->
    <DocsPySignature :symbol="symbol" class="mb-3" />

    <!-- Docstring -->
    <DocsPyDocstring v-if="symbol.docstring" :text="symbol.docstring" class="mb-3" />

    <!-- Parameters -->
    <div v-if="symbol.parameters && symbol.parameters.length" class="mb-3">
      <p
        class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
      >
        Parameters
      </p>
      <div class="border-l-2 border-subtle pl-3 space-y-2">
        <div
          v-for="param in symbol.parameters?.filter((p) => p.name !== 'self' && p.name !== 'cls')"
          :key="param.name"
        >
          <span class="font-mono text-[11px] text-sky-400">{{ param.name }}</span>
          <span v-if="param.type" class="ml-1.5 text-[10px] text-zinc-400 dark:text-zinc-600">{{
            param.type
          }}</span>
          <p v-if="param.description" class="mt-0.5 text-[11px] text-muted">
            {{ param.description }}
          </p>
        </div>
      </div>
    </div>

    <!-- Returns -->
    <div v-if="symbol.returns" class="mb-3">
      <p
        class="mb-1 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
      >
        Returns
      </p>
      <span v-if="symbol.returns.type" class="font-mono text-[11px] text-sky-400">{{
        symbol.returns.type
      }}</span>
      <span v-if="symbol.returns.description" class="ml-2 text-[11px] text-muted">{{
        symbol.returns.description
      }}</span>
    </div>

    <!-- Raises -->
    <div v-if="symbol.raises && symbol.raises.length" class="mb-3">
      <p
        class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
      >
        Raises
      </p>
      <div class="border-l-2 border-subtle pl-3 space-y-2">
        <div v-for="r in symbol.raises" :key="r.type">
          <span class="font-mono text-[11px] text-red-400">{{ r.type }}</span>
          <p v-if="r.description" class="mt-0.5 text-[11px] text-muted">{{ r.description }}</p>
        </div>
      </div>
    </div>

    <!-- Methods (classes only) -->
    <div v-if="symbol.kind === 'class' && symbol.methods && symbol.methods.length" class="mb-3">
      <button
        class="flex items-center gap-1.5 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600 hover:text-zinc-600 dark:hover:text-zinc-400 transition-colors"
        @click="toggleExpand"
      >
        <span>Methods ({{ symbol.methods.length }})</span>
        <span class="text-[10px]">{{ expanded ? "▾" : "▸" }}</span>
      </button>

      <div v-if="expanded" class="mt-3 space-y-6 border-l-2 border-subtle pl-4">
        <div v-for="method in symbol.methods" :key="method.name">
          <!-- Method name + kind badge -->
          <div class="mb-2 flex items-center gap-2">
            <span class="font-mono text-[12px] font-semibold text-primary">{{ method.name }}</span>
            <span
              class="rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide bg-blue-950 text-blue-300"
              >method</span
            >
          </div>

          <DocsPySignature :symbol="method" class="mb-2" />

          <DocsPyDocstring v-if="method.docstring" :text="method.docstring" class="mb-2" />

          <!-- Method parameters -->
          <div
            v-if="
              method.parameters &&
              method.parameters.filter((p) => p.name !== 'self' && p.name !== 'cls').length
            "
            class="mb-2"
          >
            <p
              class="mb-1 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
            >
              Parameters
            </p>
            <div class="border-l-2 border-subtle pl-3 space-y-1.5">
              <div
                v-for="param in method.parameters.filter(
                  (p) => p.name !== 'self' && p.name !== 'cls',
                )"
                :key="param.name"
              >
                <span class="font-mono text-[11px] text-sky-400">{{ param.name }}</span>
                <span
                  v-if="param.type"
                  class="ml-1.5 text-[10px] text-zinc-400 dark:text-zinc-600"
                  >{{ param.type }}</span
                >
                <p v-if="param.description" class="mt-0.5 text-[11px] text-muted">
                  {{ param.description }}
                </p>
              </div>
            </div>
          </div>

          <!-- Method returns -->
          <div v-if="method.returns" class="mb-2">
            <p
              class="mb-1 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
            >
              Returns
            </p>
            <span v-if="method.returns.type" class="font-mono text-[11px] text-sky-400">{{
              method.returns.type
            }}</span>
            <span v-if="method.returns.description" class="ml-2 text-[11px] text-muted">{{
              method.returns.description
            }}</span>
          </div>

          <!-- Method raises -->
          <div v-if="method.raises && method.raises.length" class="mb-2">
            <p
              class="mb-1 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
            >
              Raises
            </p>
            <div class="border-l-2 border-subtle pl-3 space-y-1.5">
              <div v-for="r in method.raises" :key="r.type">
                <span class="font-mono text-[11px] text-red-400">{{ r.type }}</span>
                <p v-if="r.description" class="mt-0.5 text-[11px] text-muted">
                  {{ r.description }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="mt-8 border-t border-base" />
  </div>
</template>
