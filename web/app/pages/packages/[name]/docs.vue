<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

const route = useRoute();
const name = computed(() => route.params.name as string);

const api = useApi();

const { data: pkg, status: pkgStatus } = await useAsyncData(`package-${name.value}`, () =>
  api.fetchPackage(name.value),
);

const { data: docs, status: docsStatus } = useAsyncData(
  `docs-data-${name.value}`,
  () => api.fetchDocs(name.value).catch(() => null),
  { server: false, default: () => null },
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

function symId(name: string) {
  return `sym-${encodeURIComponent(name)}`;
}

const allSymbols = computed<DocSymbol[]>(() => [
  ...allFunctions.value,
  ...allClasses.value,
  ...allExceptions.value,
]);

const paletteSections = computed(() => [
  ...(allFunctions.value.length
    ? [
        {
          label: "Functions",
          kind: "functions",
          count: allFunctions.value.length,
          firstSymbol: allFunctions.value[0]?.name ?? null,
        },
      ]
    : []),
  ...(allClasses.value.length
    ? [
        {
          label: "Classes",
          kind: "classes",
          count: allClasses.value.length,
          firstSymbol: allClasses.value[0]?.name ?? null,
        },
      ]
    : []),
  ...(allExceptions.value.length
    ? [
        {
          label: "Exceptions",
          kind: "exceptions",
          count: allExceptions.value.length,
          firstSymbol: allExceptions.value[0]?.name ?? null,
        },
      ]
    : []),
]);

// ── Deferred rendering ─────────────────────────────────────────────────────
// Safari does not support requestIdleCallback — fall back to setTimeout
const ric: typeof requestIdleCallback =
  typeof requestIdleCallback !== "undefined"
    ? requestIdleCallback
    : (cb) =>
        setTimeout(
          () => cb({ didTimeout: false, timeRemaining: () => 16 } as IdleDeadline),
          1,
        ) as unknown as number;
const cic: typeof cancelIdleCallback =
  typeof cancelIdleCallback !== "undefined"
    ? cancelIdleCallback
    : (id) => clearTimeout(id as unknown as number);

const BATCH_SIZE = 20;
const renderedCount = ref(0);
let pendingIdle: ReturnType<typeof requestIdleCallback> | null = null;

function scheduleNextBatch() {
  if (pendingIdle) cic(pendingIdle);
  pendingIdle = ric(
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
    const el = document.getElementById(symId(sym.name));
    if (el) {
      el.dataset.symbol = sym.name;
      observer!.observe(el);
    }
  }
});

onUnmounted(() => {
  observer?.disconnect();
  if (pendingIdle) cic(pendingIdle);
});

// ── Jump to symbol ─────────────────────────────────────────────────────────
async function jumpToSymbol(symbolName: string) {
  const targetIndex = allSymbols.value.findIndex((s) => s.name === symbolName);
  if (targetIndex === -1) return;

  if (targetIndex >= renderedCount.value) {
    if (pendingIdle) cic(pendingIdle);
    renderedCount.value = targetIndex + 1;
    await nextTick();
  }

  activeSymbol.value = symbolName;
  const el = document.getElementById(symId(symbolName));
  if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

// ── ⌘K / Ctrl+K global shortcut ──────────────────────────────────────────
const paletteOpen = ref(false);

function onKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    // stopImmediatePropagation prevents the app-wide CommandPalette from also opening
    e.stopImmediatePropagation();
    paletteOpen.value = !paletteOpen.value;
  }
}

onMounted(() => window.addEventListener("keydown", onKeydown, { capture: true }));
onUnmounted(() => window.removeEventListener("keydown", onKeydown, { capture: true }));

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
      <!-- Docs context bar -->
      <div class="-mx-4 mb-0 border-b border-subtle bg-base/90 backdrop-blur-sm sm:-mx-6 lg:-mx-8">
        <div class="mx-auto flex max-w-6xl items-center gap-3 px-4 py-2">
          <NuxtLink
            :to="`/packages/${pkg.name}`"
            class="flex items-center gap-1 text-sm font-medium text-[var(--color-brand)] transition-colors hover:text-[var(--color-brand-light)]"
          >
            <span>←</span>
            <span>{{ pkg.name }}</span>
          </NuxtLink>
          <span class="text-muted">·</span>
          <span class="font-mono text-xs text-muted">v{{ pkg.version }}</span>
          <span v-if="pkg.summary" class="hidden truncate text-xs text-muted sm:block">
            {{ pkg.summary }}
          </span>
          <button
            class="ml-auto flex cursor-pointer items-center gap-1.5 rounded-md border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] px-2.5 py-1.5 text-[11px] text-[var(--color-brand)] transition-colors hover:bg-[rgba(74,222,128,0.16)]"
            @click="paletteOpen = true"
          >
            <svg
              class="h-3 w-3 flex-shrink-0"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
              />
            </svg>
            <span>Jump to symbol</span>
            <kbd class="text-[9px] text-muted">⌘K</kbd>
          </button>
        </div>
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
          :sections="paletteSections"
          @jump="jumpToSymbol"
          @close="paletteOpen = false"
        />
      </ClientOnly>
    </div>
  </div>
</template>
