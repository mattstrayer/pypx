<script setup lang="ts">
import Fuse from "fuse.js";
import type { DocSymbol } from "~/types/api";

interface PaletteSection {
  label: string;
  kind: string;
  count: number;
  firstSymbol: string | null;
}

const props = defineProps<{
  symbols: DocSymbol[];
  open: boolean;
  sections: PaletteSection[];
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

type PaletteResult =
  | { type: "section"; section: PaletteSection }
  | { type: "symbol"; symbol: DocSymbol };

const results = computed<PaletteResult[]>(() => {
  const q = query.value.trim().toLowerCase();

  // Sections: simple label match
  const matchedSections = props.sections.filter((s) => !q || s.label.toLowerCase().includes(q));

  // Symbols: fuse.js when query present, grouped by kind when empty
  let matchedSymbols: DocSymbol[];
  if (!q) {
    const functions = props.symbols.filter((s) => s.kind === "function").slice(0, MAX_PER_GROUP);
    const classes = props.symbols.filter((s) => s.kind === "class").slice(0, MAX_PER_GROUP);
    const exceptions = props.symbols.filter((s) => s.kind === "exception").slice(0, MAX_PER_GROUP);
    matchedSymbols = [...functions, ...classes, ...exceptions];
  } else {
    matchedSymbols = fuse.value.search(query.value).map((r) => r.item);
  }

  return [
    ...matchedSections.map((s) => ({ type: "section" as const, section: s })),
    ...matchedSymbols.map((s) => ({ type: "symbol" as const, symbol: s })),
  ];
});

watch(query, () => {
  selectedIndex.value = 0;
});

function select(item: PaletteResult) {
  if (item.type === "section") {
    if (item.section.firstSymbol) emit("jump", item.section.firstSymbol);
  } else {
    emit("jump", item.symbol.name);
  }
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
    select(results.value[selectedIndex.value]!);
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
        role="dialog"
        aria-modal="true"
        aria-label="Documentation search"
        class="fixed left-1/2 top-[20vh] z-50 w-full max-w-lg -translate-x-1/2 rounded-xl border border-subtle bg-surface shadow-2xl overflow-hidden"
      >
        <!-- Search input -->
        <div class="flex items-center gap-3 border-b border-subtle px-4 py-3">
          <svg
            class="h-4 w-4 flex-shrink-0 text-muted"
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
            role="combobox"
            aria-label="Jump to symbol"
            aria-controls="palette-results"
            aria-autocomplete="list"
            :aria-expanded="results.length > 0"
            :aria-activedescendant="
              results[selectedIndex] ? `palette-opt-${selectedIndex}` : undefined
            "
            placeholder="Jump to symbol..."
            class="flex-1 bg-transparent font-mono text-sm text-primary placeholder-muted outline-none"
            @keydown="onKeydown"
          />
          <kbd class="text-[10px] text-muted bg-raised border border-subtle rounded px-1.5 py-0.5"
            >esc</kbd
          >
        </div>

        <!-- Results -->
        <div
          id="palette-results"
          role="listbox"
          aria-label="Symbols"
          class="max-h-80 overflow-y-auto py-1"
        >
          <div v-if="results.length === 0" class="px-4 py-6 text-center text-sm text-muted">
            No symbols found
          </div>
          <template
            v-for="(item, i) in results"
            :key="item.type === 'section' ? `sec-${item.section.kind}` : item.symbol.name"
          >
            <!-- Section item -->
            <button
              v-if="item.type === 'section'"
              :id="`palette-opt-${i}`"
              role="option"
              :aria-selected="i === selectedIndex"
              data-testid="palette-section"
              class="flex w-full items-center gap-3 px-4 py-2 text-left transition-colors"
              :class="i === selectedIndex ? 'bg-raised' : 'hover:bg-raised/50'"
              @click="select(item)"
              @mouseover="selectedIndex = i"
            >
              <span
                class="flex-shrink-0 rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide bg-raised text-primary"
                >#</span
              >
              <span class="font-mono text-sm text-primary">{{ item.section.label }}</span>
              <span class="ml-auto text-[10px] text-muted">{{ item.section.count }}</span>
            </button>
            <!-- Symbol item -->
            <button
              v-else
              :id="`palette-opt-${i}`"
              role="option"
              :aria-selected="i === selectedIndex"
              data-testid="palette-result"
              class="flex w-full items-center gap-3 px-4 py-2 text-left transition-colors"
              :class="i === selectedIndex ? 'bg-raised' : 'hover:bg-raised/50'"
              @click="select(item)"
              @mouseover="selectedIndex = i"
            >
              <span
                class="flex-shrink-0 rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
                :class="{
                  'bg-blue-950 text-blue-300': item.symbol.kind === 'function',
                  'bg-purple-950 text-purple-300': item.symbol.kind === 'class',
                  'bg-red-950 text-red-300': item.symbol.kind === 'exception',
                }"
                >{{ item.symbol.kind }}</span
              >
              <span class="font-mono text-sm text-primary">{{ item.symbol.name }}</span>
            </button>
          </template>
        </div>

        <!-- Footer hint -->
        <div
          class="border-t border-subtle px-4 py-2 flex items-center gap-3 text-[10px] text-muted"
        >
          <span
            ><kbd class="bg-raised border border-subtle rounded px-1 py-0.5">↑↓</kbd> navigate</span
          >
          <span><kbd class="bg-raised border border-subtle rounded px-1 py-0.5">↵</kbd> jump</span>
          <span
            ><kbd class="bg-raised border border-subtle rounded px-1 py-0.5">esc</kbd> close</span
          >
        </div>
      </div>
    </div>
  </Teleport>
</template>
