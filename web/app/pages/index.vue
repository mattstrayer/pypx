<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, close } =
  useSearchTypeahead();
const searchWrapper = ref<HTMLElement | null>(null);

function onSearch() {
  // Search is handled via the typeahead dropdown; form submit is a no-op.
}

// Close dropdown when clicking outside
function onClickOutside(e: MouseEvent) {
  if (searchWrapper.value && !searchWrapper.value.contains(e.target as Node)) {
    close();
  }
}

onMounted(() => {
  document.addEventListener("mousedown", onClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("mousedown", onClickOutside);
});

const POPULAR_LIMIT = 24;

const api = useApi();
const { data: trendingPackages, status } = await useAsyncData("trending", () =>
  api.fetchPopular(POPULAR_LIMIT),
);

useHead({ titleTemplate: "%s" });
useSeoMeta({
  title: "pypx — A Modern PyPI Frontend",
  description:
    "Explore Python packages with enriched insights, fast search, dependency analysis, and download trends.",
  ogTitle: "pypx — A Modern PyPI Frontend",
  ogDescription:
    "Explore Python packages with enriched insights, fast search, dependency analysis, and download trends.",
});

defineOgImage("SiteCard", {}, { width: 1200, height: 630 });
</script>

<template>
  <div>
    <section class="flex flex-col items-center pt-16 pb-12 text-center">
      <!-- Eyebrow pill -->
      <div
        class="mb-6 inline-flex items-center gap-2 rounded-full border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] px-3 py-1.5 text-xs font-medium text-[var(--color-brand)]"
      >
        <span
          class="h-1.5 w-1.5 rounded-full bg-[var(--color-brand)] animate-pulse"
          aria-hidden="true"
        />
        500,000+ packages indexed
      </div>

      <h1
        class="text-[clamp(2.5rem,6vw,3.25rem)] font-bold tracking-[-0.04em] text-primary leading-[1.05]"
      >
        The Python Package<br />
        <span class="text-[var(--color-brand)]">Index, reimagined.</span>
      </h1>

      <p class="mt-4 max-w-lg text-lg text-muted leading-relaxed">
        Fast search, dependency insights, API docs, and security advisories — all in one place.
      </p>

      <div ref="searchWrapper" class="relative mt-8 w-full max-w-[560px]">
        <form @submit.prevent="onSearch">
          <div class="relative">
            <input
              v-model="query"
              type="text"
              aria-label="Search Python packages"
              placeholder="Search packages..."
              class="w-full rounded-xl border border-subtle bg-surface px-4 py-3.5 pr-16 text-primary placeholder-muted outline-none transition-[border-color,box-shadow] focus:border-[var(--color-brand-border)] focus:ring-2 focus:ring-[var(--color-brand-border)] focus:ring-offset-0 shadow-[0_1px_3px_rgba(0,0,0,0.15)]"
              @keydown="onKeydown"
              @focus="query.trim() && (isOpen = true)"
            />
            <kbd
              class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 rounded border border-subtle bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted"
            >
              ⌘K
            </kbd>
          </div>
        </form>

        <!-- Typeahead dropdown -->
        <div v-if="isOpen" class="absolute top-full left-0 right-0 z-50 mt-1">
          <SearchDropdown
            :results="results"
            :selected-index="selectedIndex"
            :loading="isLoading"
            :has-query="!!query.trim()"
            @select="navigateToResult"
            @hover="(i) => (selectedIndex = i)"
          />
        </div>
      </div>

      <p class="mt-2.5 text-xs text-muted">↑↓ to navigate · ↵ to open · esc to close</p>
    </section>

    <!-- Feature strip — API docs, security, dependencies -->
    <div
      class="mb-4 mt-16 grid grid-cols-1 gap-px overflow-hidden rounded-[14px] border border-subtle bg-subtle sm:grid-cols-3"
    >
      <div class="flex flex-col gap-2 bg-surface px-6 py-5 transition-colors hover:bg-raised">
        <div
          aria-hidden="true"
          class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] text-base"
        >
          📦
        </div>
        <p class="text-sm font-semibold text-primary">API Documentation</p>
        <p class="text-xs leading-relaxed text-muted">
          Browse extracted docs from any package — functions, classes, type signatures, and
          docstrings.
        </p>
      </div>
      <div class="flex flex-col gap-2 bg-surface px-6 py-5 transition-colors hover:bg-raised">
        <div
          aria-hidden="true"
          class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] text-base"
        >
          🔒
        </div>
        <p class="text-sm font-semibold text-primary">Security Advisories</p>
        <p class="text-xs leading-relaxed text-muted">
          CVE and vulnerability data from OSV.dev. Know if a package has known issues before you
          install.
        </p>
      </div>
      <div class="flex flex-col gap-2 bg-surface px-6 py-5 transition-colors hover:bg-raised">
        <div
          aria-hidden="true"
          class="flex h-8 w-8 items-center justify-center rounded-lg border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] text-base"
        >
          🌿
        </div>
        <p class="text-sm font-semibold text-primary">Dependency Analysis</p>
        <p class="text-xs leading-relaxed text-muted">
          Full dependency tree with optional extras, platform coverage, and install size estimates.
        </p>
      </div>
    </div>

    <section class="pb-16">
      <div class="mb-4 flex items-center gap-3">
        <span class="text-xs font-semibold uppercase tracking-[0.07em] text-muted">Trending</span>
        <div class="h-px flex-1 bg-subtle" />
        <span class="font-mono text-[11.5px] text-muted opacity-70">
          top 24 by downloads · data from
          <a
            href="https://github.com/hugovk/top-pypi-packages"
            target="_blank"
            rel="noopener noreferrer"
            class="underline decoration-dotted underline-offset-2 hover:text-primary"
            >hugovk/top-pypi-packages</a
          >
        </span>
      </div>

      <!-- Skeleton loading state -->
      <div
        v-if="status === 'pending'"
        class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-3"
      >
        <div
          v-for="i in POPULAR_LIMIT"
          :key="i"
          class="h-[90px] animate-pulse rounded-[10px] border border-subtle bg-raised/50"
        />
      </div>

      <!-- Error state -->
      <p v-else-if="status === 'error'" class="text-sm text-muted">
        Could not load popular packages.
      </p>

      <!-- Data -->
      <TrendingPackages v-else-if="trendingPackages?.length" :packages="trendingPackages" />
      <p v-else class="text-sm text-muted">No trending packages available.</p>
    </section>
  </div>
</template>
