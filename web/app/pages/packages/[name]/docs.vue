<script setup lang="ts">
import type { DocEntry } from "~/utils/docSymbols";

const route = useRoute();
const name = computed(() => route.params.name as string);

const api = useApi();

const {
  data: pkg,
  status: pkgStatus,
  error: pkgError,
} = await useAsyncData(
  () => `package-${name.value}`,
  () => api.fetchPackage(name.value),
  { watch: [name] },
);

const pageStatus = computed(() => pageStatusForApiError(pkgError.value));

const { data: docs, status: docsStatus } = useAsyncData(
  () => `docs-data-${name.value}`,
  () => api.fetchDocs(name.value).catch(() => null),
  { server: false, default: () => null, watch: [name] },
);

const unavailableDetail = computed(() => {
  switch (docs.value?.reason) {
    case "no_artifact":
      return "This release has no wheel, and pypx found no readable Python source in its files.";
    case "too_large":
      return "This release is too large for pypx to read.";
    default:
      return "This package may be binary-only or could not be parsed.";
  }
});

const entries = computed(() => docEntries(docs.value?.modules ?? []));
const allFunctions = computed(() => entries.value.functions);
const allClasses = computed(() => entries.value.classes);
const allExceptions = computed(() => entries.value.exceptions);
const allConstants = computed(() => entries.value.constants);
const reExports = computed(() => (docs.value?.modules ?? []).filter((m) => m.aliases?.length));
const reExportCount = computed(() =>
  reExports.value.reduce((n, m) => n + (m.aliases?.length ?? 0), 0),
);

// The DOM ids of the sections after the cards. Jumps use them in place of a symbol id.
const CONSTANTS_SECTION = "constants";
const RE_EXPORTS_SECTION = "re-exports";

function symId(id: string) {
  return `sym-${encodeURIComponent(id)}`;
}

const allSymbols = computed<DocEntry[]>(() => [
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
          firstSymbol: allFunctions.value[0]?.id ?? null,
        },
      ]
    : []),
  ...(allClasses.value.length
    ? [
        {
          label: "Classes",
          kind: "classes",
          count: allClasses.value.length,
          firstSymbol: allClasses.value[0]?.id ?? null,
        },
      ]
    : []),
  ...(allExceptions.value.length
    ? [
        {
          label: "Exceptions",
          kind: "exceptions",
          count: allExceptions.value.length,
          firstSymbol: allExceptions.value[0]?.id ?? null,
        },
      ]
    : []),
  ...(allConstants.value.length
    ? [
        {
          label: "Constants",
          kind: "constants",
          count: allConstants.value.length,
          firstSymbol: CONSTANTS_SECTION,
        },
      ]
    : []),
  ...(reExportCount.value
    ? [
        {
          label: "Re-exports",
          kind: "re-exports",
          count: reExportCount.value,
          firstSymbol: RE_EXPORTS_SECTION,
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

// The sections after the cards wait for the last card, so a deferred card never
// renders above them.
const cardsRendered = computed(() => renderedCount.value >= allSymbols.value.length);

// ── Active symbol + scroll tracking ───────────────────────────────────────
const activeSymbol = ref<string | null>(null);
let observer: IntersectionObserver | null = null;

// A jump scrolls past other cards and rows, and it stops with the target above the
// band the observer watches. The target stays active until that scroll ends.
const MAX_JUMP_SCROLL_MS = 3000;
let jumping = false;
let jumpTimer: ReturnType<typeof setTimeout> | undefined;

function endJump() {
  jumping = false;
  clearTimeout(jumpTimer);
  window.removeEventListener("scrollend", endJump);
}

function startJump() {
  endJump();
  jumping = true;
  window.addEventListener("scrollend", endJump);
  // A target already in place scrolls nothing, so no scrollend event comes.
  jumpTimer = setTimeout(endJump, MAX_JUMP_SCROLL_MS);
}

function setupObserver() {
  observer?.disconnect();
  observer = new IntersectionObserver(
    (entries) => {
      if (jumping) return;
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
    const el = document.getElementById(symId(sym.id));
    if (el) {
      el.dataset.symbol = sym.id;
      observer!.observe(el);
    }
  }
});

// Constant rows take part in the scroll tracking as cards do.
watch(
  [cardsRendered, allConstants],
  async ([rendered]) => {
    if (!import.meta.client || !rendered) return;
    await nextTick();
    if (!observer) setupObserver();
    for (const el of document.querySelectorAll("[data-testid='constant-row']")) {
      observer!.observe(el);
    }
  },
  { immediate: true },
);

onUnmounted(() => {
  observer?.disconnect();
  if (pendingIdle) cic(pendingIdle);
  endJump();
});

// ── Jump to symbol ─────────────────────────────────────────────────────────
async function jumpToSymbol(symbolId: string) {
  const isSection = symbolId === CONSTANTS_SECTION || symbolId === RE_EXPORTS_SECTION;
  const cardIndex = allSymbols.value.findIndex((s) => s.id === symbolId);
  if (!isSection && cardIndex === -1 && !allConstants.value.some((c) => c.id === symbolId)) return;

  // A target after the cards needs every card rendered above it.
  const needed = cardIndex === -1 ? allSymbols.value.length : cardIndex + 1;
  if (needed > renderedCount.value) {
    if (pendingIdle) cic(pendingIdle);
    renderedCount.value = needed;
    // The batches stopped above, and the cards below the target still need them.
    if (needed < allSymbols.value.length) scheduleNextBatch();
    await nextTick();
  }

  // A section has no row in the sidebar to mark.
  if (!isSection) activeSymbol.value = symbolId;
  const el = document.getElementById(isSection ? symbolId : symId(symbolId));
  if (el) {
    startJump();
    el.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

// ── ⌘K / Ctrl+K global shortcut ──────────────────────────────────────────
const paletteOpen = ref(false);

function onKeydown(e: KeyboardEvent) {
  // With no symbols to jump to, the app-wide palette keeps the shortcut.
  if (docs.value?.available && (e.metaKey || e.ctrlKey) && e.key === "k") {
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
  title: () => {
    if (pkg.value) return `${pkg.value.name} API Docs`;
    if (pageStatus.value === 404) return "Package not found";
    return pageStatus.value ? "Package unavailable" : "Loading";
  },
  description: () => `API documentation for ${pkg.value?.name ?? name.value}`,
});

if (pkg.value) {
  defineOgImage(
    "DocsCard",
    {
      name: () => pkg.value?.name ?? "",
      version: () => pkg.value?.version ?? "",
    },
    { width: 1200, height: 630 },
  );
}
</script>

<template>
  <div>
    <!-- Package loading -->
    <div v-if="pkgStatus === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Package error -->
    <PackageLoadError v-else-if="pageStatus" :status="pageStatus" :name="name" />

    <div v-else-if="pkg">
      <h1 class="sr-only">{{ pkg.name }} API documentation</h1>
      <!-- Docs context bar. The bleed matches the layout: main pads 1rem, and it has
           side margins to spare only once the viewport passes its 72rem width + 2rem. -->
      <div class="-mx-4 mb-0 border-b border-subtle bg-base/90 backdrop-blur-sm min-[74rem]:-mx-8">
        <div class="mx-auto flex max-w-6xl items-center gap-3 px-4 py-2">
          <NuxtLink
            :to="`/packages/${pkg.name}`"
            class="flex min-w-0 items-center gap-1 text-sm font-medium text-[var(--color-brand)] transition-colors hover:text-[var(--color-brand-light)]"
          >
            <span>←</span>
            <span class="truncate">{{ pkg.name }}</span>
          </NuxtLink>
          <span class="text-muted">·</span>
          <span class="shrink-0 font-mono text-xs text-muted">v{{ pkg.version }}</span>
          <span v-if="pkg.summary" class="hidden truncate text-xs text-muted sm:block">
            {{ pkg.summary }}
          </span>
          <button
            v-if="docs?.available"
            class="ml-auto flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] px-2.5 py-1.5 text-[11px] text-[var(--color-brand)] transition-colors hover:bg-[rgba(74,222,128,0.16)]"
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
          <p class="mt-1 text-sm text-muted">{{ unavailableDetail }}</p>
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
        <div v-else class="flex gap-0 -mx-4 min-[74rem]:-mx-8">
          <!-- Virtual sidebar -->
          <DocsSidebar
            :functions="allFunctions"
            :classes="allClasses"
            :exceptions="allExceptions"
            :constants="allConstants"
            :re-export-count="reExportCount"
            :active-symbol="activeSymbol"
            @select="jumpToSymbol"
          />

          <!-- Main content. Below lg a long name or type breaks where it would widen the
               page. It inherits to every card, and code blocks stay on their lines. -->
          <div class="flex-1 min-w-0 px-6 py-5 max-lg:[overflow-wrap:anywhere]">
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
              :key="sym.id"
              :symbol="sym"
              :is-active="activeSymbol === sym.id"
            />

            <!-- Render progress indicator -->
            <DocsSkeletonLoader
              v-if="renderedCount < allSymbols.length"
              mode="render"
              :rendered-count="renderedCount"
              :total-count="allSymbols.length"
            />

            <DocsConstants
              v-if="allConstants.length && cardsRendered"
              :constants="allConstants"
              :active-symbol="activeSymbol"
            />

            <DocsReExports v-if="reExports.length && cardsRendered" :modules="reExports" />
          </div>
        </div>

        <!-- ⌘K Command Palette -->
        <DocsCommandPalette
          :symbols="allSymbols"
          :aliases="entries.aliases"
          :constants="allConstants"
          :open="paletteOpen"
          :sections="paletteSections"
          @jump="jumpToSymbol"
          @close="paletteOpen = false"
        />
      </ClientOnly>
    </div>
  </div>
</template>
