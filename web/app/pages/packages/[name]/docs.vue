<!-- web/app/pages/packages/[name]/docs.vue -->
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

const activeSymbol = ref<string | null>(null);

function scrollTo(symbolName: string) {
  activeSymbol.value = symbolName;
  const el = document.getElementById(`sym-${symbolName}`);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "start" });
  }
}

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
    <!-- Loading state -->
    <div v-if="pkgStatus === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Error state -->
    <div v-else-if="pkgStatus === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">Package not found</p>
    </div>

    <!-- Loaded -->
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

      <!-- Tab strip — Docs tab active, others link back to package page -->
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

      <!-- Docs loading -->
      <div v-if="docsStatus === 'pending'" class="flex items-center justify-center py-16">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-subtle border-t-primary" />
      </div>

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

      <!-- Docs content: sidebar + main -->
      <div v-else class="flex gap-0 -mx-4 sm:-mx-6 lg:-mx-8">
        <!-- Fixed sidebar -->
        <div
          class="w-48 flex-shrink-0 sticky top-0 h-screen overflow-y-auto border-r border-subtle bg-base py-3 hidden md:block"
        >
          <p
            class="px-3 pb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
          >
            Contents
          </p>

          <!-- Functions -->
          <template v-if="allFunctions.length">
            <p class="px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
              Functions
              <span class="text-zinc-400 dark:text-zinc-700">({{ allFunctions.length }})</span>
            </p>
            <button
              v-for="sym in allFunctions"
              :key="sym.name"
              class="block w-full px-4 py-1 text-left text-[11px] font-mono transition-colors"
              :class="
                activeSymbol === sym.name
                  ? 'border-r-2 border-[var(--color-brand)] bg-[var(--color-brand)]/5 text-[var(--color-brand)]'
                  : 'text-muted hover:text-zinc-700 dark:hover:text-zinc-300'
              "
              @click="scrollTo(sym.name)"
            >
              {{ sym.name }}
            </button>
          </template>

          <!-- Classes -->
          <template v-if="allClasses.length">
            <p class="px-3 pt-3 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
              Classes
              <span class="text-zinc-400 dark:text-zinc-700">({{ allClasses.length }})</span>
            </p>
            <button
              v-for="sym in allClasses"
              :key="sym.name"
              class="block w-full px-4 py-1 text-left text-[11px] font-mono transition-colors"
              :class="
                activeSymbol === sym.name
                  ? 'border-r-2 border-[var(--color-brand)] bg-[var(--color-brand)]/5 text-[var(--color-brand)]'
                  : 'text-muted hover:text-zinc-700 dark:hover:text-zinc-300'
              "
              @click="scrollTo(sym.name)"
            >
              {{ sym.name }}
            </button>
          </template>

          <!-- Exceptions -->
          <template v-if="allExceptions.length">
            <p class="px-3 pt-3 pb-1 text-[10px] font-semibold uppercase tracking-wide text-muted">
              Exceptions
              <span class="text-zinc-400 dark:text-zinc-700">({{ allExceptions.length }})</span>
            </p>
            <button
              v-for="sym in allExceptions"
              :key="sym.name"
              class="block w-full px-4 py-1 text-left text-[11px] font-mono transition-colors"
              :class="
                activeSymbol === sym.name
                  ? 'border-r-2 border-[var(--color-brand)] bg-[var(--color-brand)]/5 text-[var(--color-brand)]'
                  : 'text-muted hover:text-zinc-700 dark:hover:text-zinc-300'
              "
              @click="scrollTo(sym.name)"
            >
              {{ sym.name }}
            </button>
          </template>
        </div>

        <!-- Main content -->
        <div class="flex-1 min-w-0 px-6 py-5">
          <template
            v-for="sym in [...allFunctions, ...allClasses, ...allExceptions]"
            :key="sym.name"
          >
            <div :id="`sym-${sym.name}`" class="mb-10 scroll-mt-4">
              <!-- Symbol name + kind badge -->
              <div class="mb-3 flex items-center gap-2">
                <span class="font-mono text-base font-bold text-primary">{{ sym.name }}</span>
                <span
                  class="rounded px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wide"
                  :class="{
                    'bg-blue-950 text-blue-300': sym.kind === 'function',
                    'bg-purple-950 text-purple-300': sym.kind === 'class',
                    'bg-red-950 text-red-300': sym.kind === 'exception',
                  }"
                  >{{ sym.kind }}</span
                >
              </div>

              <!-- Signature (semantic highlighting) -->
              <DocsPySignature :symbol="sym" class="mb-3" />

              <!-- Docstring (formatted with code highlighting) -->
              <DocsPyDocstring v-if="sym.docstring" :text="sym.docstring" class="mb-3" />

              <!-- Parameters -->
              <div v-if="sym.parameters && sym.parameters.length" class="mb-3">
                <p
                  class="mb-2 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
                >
                  Parameters
                </p>
                <div class="border-l-2 border-subtle pl-3 space-y-2">
                  <div
                    v-for="param in sym.parameters?.filter(
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

              <!-- Returns -->
              <div v-if="sym.returns" class="mb-3">
                <p
                  class="mb-1 text-[9px] font-bold uppercase tracking-widest text-zinc-400 dark:text-zinc-600"
                >
                  Returns
                </p>
                <span v-if="sym.returns.type" class="font-mono text-[11px] text-sky-400">{{
                  sym.returns.type
                }}</span>
                <span v-if="sym.returns.description" class="ml-2 text-[11px] text-muted">{{
                  sym.returns.description
                }}</span>
              </div>

              <div class="mt-8 border-t border-base" />
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
