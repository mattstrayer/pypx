<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const route = useRoute();
const name = computed(() => route.params.name as string);

const api = useApi();

const { data: pkg, status: pkgStatus } = await useAsyncData(`package-${name.value}`, () =>
  api.fetchPackage(name.value),
);

const { data: docs, status: docsStatus } = await useAsyncData(`docs-data-${name.value}`, () =>
  api.fetchDocs(name.value),
);

function dedup(symbols: DocSymbol[]): DocSymbol[] {
  const seen = new Set<string>();
  return symbols.filter((s) => {
    if (seen.has(s.name)) return false;
    seen.add(s.name);
    return true;
  });
}

const allFunctions = computed<DocSymbol[]>(() =>
  dedup(docs.value?.modules?.flatMap((m) => m.functions) ?? []),
);
const allClasses = computed<DocSymbol[]>(() =>
  dedup(docs.value?.modules?.flatMap((m) => m.classes) ?? []),
);
const allExceptions = computed<DocSymbol[]>(() =>
  dedup(docs.value?.modules?.flatMap((m) => m.exceptions) ?? []),
);

const allSymbols = computed<DocSymbol[]>(() => [
  ...allFunctions.value,
  ...allClasses.value,
  ...allExceptions.value,
]);

// ── Deferred rendering ─────────────────────────────────────────────────────
const BATCH_SIZE = 20;
const renderedCount = ref(0);
let pendingIdle: ReturnType<typeof requestIdleCallback> | null = null;

function scheduleNextBatch() {
  if (pendingIdle) cancelIdleCallback(pendingIdle);
  pendingIdle = requestIdleCallback(
    () => {
      renderedCount.value = Math.min(renderedCount.value + BATCH_SIZE, allSymbols.value.length);
      if (renderedCount.value < allSymbols.value.length) scheduleNextBatch();
    },
    { timeout: 500 },
  );
}

watch(
  allSymbols,
  (syms) => {
    if (syms.length === 0 || !import.meta.client) return;
    renderedCount.value = Math.min(BATCH_SIZE, syms.length);
    scheduleNextBatch();
  },
  { immediate: true },
);

const visibleSymbols = computed(() => allSymbols.value.slice(0, renderedCount.value));

// ── Active symbol + scroll tracking ───────────────────────────────────────
const activeSymbol = ref<string | null>(null);
let observer: IntersectionObserver | null = null;

function setupObserver() {
  observer?.disconnect();
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          activeSymbol.value = (entry.target as HTMLElement).dataset.symbol ?? null;
          break;
        }
      }
    },
    { rootMargin: "-10% 0px -80% 0px", threshold: 0 },
  );
}

watch(renderedCount, (newCount, oldCount) => {
  if (!import.meta.client) return;
  if (!observer) setupObserver();
  for (let i = oldCount; i < newCount; i++) {
    const sym = allSymbols.value[i];
    if (!sym) continue;
    const el = document.getElementById(`sym-${sym.name}`);
    if (el) {
      el.dataset.symbol = sym.name;
      observer!.observe(el);
    }
  }
});

onUnmounted(() => {
  observer?.disconnect();
  if (pendingIdle) cancelIdleCallback(pendingIdle);
});

// ── Jump to symbol ─────────────────────────────────────────────────────────
async function jumpToSymbol(symbolName: string) {
  const targetIndex = allSymbols.value.findIndex((s) => s.name === symbolName);
  if (targetIndex === -1) return;

  if (targetIndex >= renderedCount.value) {
    if (pendingIdle) cancelIdleCallback(pendingIdle);
    renderedCount.value = targetIndex + 1;
    await nextTick();
  }

  activeSymbol.value = symbolName;
  const el = document.getElementById(`sym-${symbolName}`);
  if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

// ── ⌘K / Ctrl+K global shortcut ──────────────────────────────────────────
const paletteOpen = ref(false);

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    paletteOpen.value = true;
  }
}

onMounted(() => window.addEventListener("keydown", onKeydown));
onUnmounted(() => window.removeEventListener("keydown", onKeydown));

// ── SEO ────────────────────────────────────────────────────────────────────
useSeoMeta({
  title: () => (pkg.value ? `${pkg.value.name} API Docs` : "Loading"),
  description: () => `API documentation for ${pkg.value?.name ?? name.value}`,
});

defineOgImage(
  "DocsCard",
  {
    name: () => pkg.value?.name ?? "",
    version: () => pkg.value?.version ?? "",
  },
  { width: 1200, height: 630 },
);
</script>

<template>
  <div>
    <!-- Package loading -->
    <div v-if="pkgStatus === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Package error -->
    <div v-else-if="pkgStatus === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Package not found</p>
    </div>

    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex flex-wrap items-baseline gap-3">
          <NuxtLink
            :to="`/packages/${pkg.name}`"
            class="text-3xl font-bold text-primary hover:text-zinc-700 dark:hover:text-zinc-300 transition-colors"
            >{{ pkg.name }}</NuxtLink
          >
          <span class="rounded bg-raised px-2 py-0.5 font-mono text-sm text-muted">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-muted">{{ pkg.summary }}</p>
      </div>

      <!-- Tab strip -->
      <div class="mb-6 flex gap-1 overflow-x-auto border-b border-subtle pb-0">
        <NuxtLink
          v-for="tab in ['Overview', 'Dependencies', 'Versions', 'Stats']"
          :key="tab"
          :to="`/packages/${pkg.name}`"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-muted transition-colors hover:text-zinc-700 dark:hover:text-zinc-300"
          >{{ tab }}</NuxtLink
        >
        <span
          class="cursor-default whitespace-nowrap rounded-t bg-raised px-4 py-2 text-sm font-medium text-primary"
          >Docs</span
        >
      </div>

      <!-- All docs content is client-only: virtual scroller + IntersectionObserver don't SSR -->
      <ClientOnly>
        <template #fallback>
          <DocsSkeletonLoader mode="fetch" />
        </template>

        <!-- Docs fetch skeleton -->
        <DocsSkeletonLoader v-if="docsStatus === 'pending'" mode="fetch" />

        <!-- Docs unavailable -->
        <div v-else-if="!docs?.available" class="py-16 text-center">
          <p class="text-muted">API documentation is not available for this package.</p>
          <p class="mt-1 text-sm text-muted">
            This package may be binary-only or could not be parsed.
          </p>
          <a
            v-if="pkg.doc_url"
            :href="pkg.doc_url"
            target="_blank"
            rel="noopener noreferrer"
            class="mt-4 inline-block text-sm text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
            >View external documentation →</a
          >
        </div>

        <!-- Docs content -->
        <div v-else class="flex gap-0 -mx-4 sm:-mx-6 lg:-mx-8">
          <!-- Virtual sidebar -->
          <DocsSidebar
            :functions="allFunctions"
            :classes="allClasses"
            :exceptions="allExceptions"
            :active-symbol="activeSymbol"
            @select="jumpToSymbol"
            @open-palette="paletteOpen = true"
          />

          <!-- Main content -->
          <div class="flex-1 min-w-0 px-6 py-5">
            <!-- Stub attribution -->
            <div
              v-if="docs?.stub_package"
              class="mb-5 flex items-center gap-2 rounded-md border border-subtle bg-zinc-50 px-3 py-2 text-xs text-muted dark:bg-zinc-900"
            >
              <svg
                class="h-3.5 w-3.5 flex-shrink-0 text-zinc-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M13 16h-1v-4h-1m1-4h.01M12 2a10 10 0 100 20A10 10 0 0012 2z"
                />
              </svg>
              <span
                >Type information enriched from
                <NuxtLink
                  :to="`/packages/${docs.stub_package}`"
                  class="font-mono text-[var(--color-brand)] hover:underline"
                  >{{ docs.stub_package }}</NuxtLink
                >
              </span>
            </div>

            <!-- Rendered symbols (deferred) -->
            <DocsSymbolCard
              v-for="sym in visibleSymbols"
              :key="sym.name"
              :symbol="sym"
              :is-active="activeSymbol === sym.name"
            />

            <!-- Render progress indicator -->
            <DocsSkeletonLoader
              v-if="renderedCount < allSymbols.length"
              mode="render"
              :rendered-count="renderedCount"
              :total-count="allSymbols.length"
            />
          </div>
        </div>

        <!-- ⌘K Command Palette -->
        <DocsCommandPalette
          :symbols="allSymbols"
          :open="paletteOpen"
          @jump="jumpToSymbol"
          @close="paletteOpen = false"
        />
      </ClientOnly>
    </div>
  </div>
</template>
