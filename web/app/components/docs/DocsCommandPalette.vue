<script setup lang="ts">
import Fuse from "fuse.js";
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  symbols: DocSymbol[];
  open: boolean;
}>();

const emit = defineEmits<{
  jump: [name: string];
  close: [];
}>();

const query = ref("");
const selectedIndex = ref(0);

const fuse = computed(
  () =>
    new Fuse(props.symbols, {
      keys: ["name", "kind"],
      threshold: 0.4,
      includeScore: true,
    }),
);

const MAX_PER_GROUP = 8;

const results = computed<DocSymbol[]>(() => {
  if (!query.value.trim()) {
    const functions = props.symbols.filter((s) => s.kind === "function").slice(0, MAX_PER_GROUP);
    const classes = props.symbols.filter((s) => s.kind === "class").slice(0, MAX_PER_GROUP);
    const exceptions = props.symbols.filter((s) => s.kind === "exception").slice(0, MAX_PER_GROUP);
    return [...functions, ...classes, ...exceptions];
  }
  return fuse.value.search(query.value).map((r) => r.item);
});

watch(query, () => {
  selectedIndex.value = 0;
});

function select(name: string) {
  emit("jump", name);
  emit("close");
  query.value = "";
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    emit("close");
    query.value = "";
  } else if (e.key === "ArrowDown") {
    e.preventDefault();
    selectedIndex.value = Math.min(selectedIndex.value + 1, results.value.length - 1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0);
  } else if (e.key === "Enter" && results.value[selectedIndex.value]) {
    select(results.value[selectedIndex.value].name);
  }
}

const inputRef = ref<HTMLInputElement | null>(null);
watch(
  () => props.open,
  (open) => {
    if (open) {
      query.value = "";
      selectedIndex.value = 0;
      nextTick(() => inputRef.value?.focus());
    }
  },
);

const isMac = typeof navigator !== "undefined" && /mac/i.test(navigator.platform);
const shortcutLabel = isMac ? "⌘K" : "Ctrl+K";
</script>

<template>
  <Teleport to="body">
    <div v-if="open">
      <!-- Backdrop -->
      <div
        data-testid="palette-backdrop"
        class="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm"
        @click="emit('close')"
      />

      <!-- Modal -->
      <div
        data-testid="palette-modal"
        class="fixed left-1/2 top-[20vh] z-50 w-full max-w-lg -translate-x-1/2 rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl overflow-hidden"
      >
        <!-- Search input -->
        <div class="flex items-center gap-3 border-b border-zinc-700 px-4 py-3">
          <svg
            class="h-4 w-4 flex-shrink-0 text-zinc-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
          <input
            ref="inputRef"
            v-model="query"
            data-testid="palette-input"
            type="text"
            placeholder="Jump to symbol..."
            class="flex-1 bg-transparent font-mono text-sm text-zinc-100 placeholder-zinc-600 outline-none"
            @keydown="onKeydown"
          />
          <kbd
            class="text-[10px] text-zinc-600 bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5"
            >esc</kbd
          >
        </div>

        <!-- Results -->
        <div class="max-h-80 overflow-y-auto py-1">
          <div v-if="results.length === 0" class="px-4 py-6 text-center text-sm text-zinc-600">
            No symbols found
          </div>
          <button
            v-for="(sym, i) in results"
            :key="sym.name"
            data-testid="palette-result"
            class="flex w-full items-center gap-3 px-4 py-2 text-left transition-colors"
            :class="i === selectedIndex ? 'bg-zinc-800' : 'hover:bg-zinc-800/50'"
            @click="select(sym.name)"
            @mouseover="selectedIndex = i"
          >
            <span
              class="flex-shrink-0 rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
              :class="{
                'bg-blue-950 text-blue-300': sym.kind === 'function',
                'bg-purple-950 text-purple-300': sym.kind === 'class',
                'bg-red-950 text-red-300': sym.kind === 'exception',
              }"
              >{{ sym.kind }}</span
            >
            <span class="font-mono text-sm text-zinc-200">{{ sym.name }}</span>
          </button>
        </div>

        <!-- Footer hint -->
        <div
          class="border-t border-zinc-700 px-4 py-2 flex items-center gap-3 text-[10px] text-zinc-600"
        >
          <span
            ><kbd class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5">↑↓</kbd>
            navigate</span
          >
          <span
            ><kbd class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5">↵</kbd> jump</span
          >
          <span
            ><kbd class="bg-zinc-800 border border-zinc-700 rounded px-1 py-0.5">esc</kbd>
            close</span
          >
        </div>
      </div>
    </div>
  </Teleport>
</template>
