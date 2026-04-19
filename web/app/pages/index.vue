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
      <h1 class="text-5xl font-bold tracking-tight text-[var(--color-brand)]">pypx</h1>
      <p class="mt-3 max-w-lg text-lg text-muted">
        The Python Package Index, reimagined. Fast search, dependency insights, and download trends
        — all in one place.
      </p>
      <div ref="searchWrapper" class="relative mt-8 w-full max-w-xl">
        <form @submit.prevent="onSearch">
          <div class="relative">
            <input
              v-model="query"
              type="text"
              aria-label="Search Python packages"
              placeholder="Search 500,000+ Python packages..."
              class="w-full rounded-lg border border-subtle bg-surface px-4 py-3 pr-16 text-primary placeholder-muted outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
              @keydown="onKeydown"
              @focus="query.trim() && (isOpen = true)"
            />
            <kbd
              class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 hidden rounded bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline"
            >
              /
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
    </section>

    <!-- Feature callout strip -->
    <section class="pb-10">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <div class="mb-2 flex items-center gap-2">
            <svg
              class="h-4 w-4 text-[var(--color-brand)]"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 7h16M4 12h8m-8 5h16" />
            </svg>
            <span class="text-sm font-semibold text-primary">Dependency Insights</span>
          </div>
          <p class="text-sm text-muted">
            Required deps, optional extras, and Python version constraints at a glance.
          </p>
        </div>
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <div class="mb-2 flex items-center gap-2">
            <svg
              class="h-4 w-4 text-[var(--color-brand)]"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 17l4-8 4 4 4-6 4 3" />
            </svg>
            <span class="text-sm font-semibold text-primary">Download Trends</span>
          </div>
          <p class="text-sm text-muted">
            Weekly and monthly download stats with historical charts.
          </p>
        </div>
        <div class="rounded-lg border border-subtle bg-surface p-4">
          <div class="mb-2 flex items-center gap-2">
            <svg
              class="h-4 w-4 text-[var(--color-brand)]"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M9 12h6m-6 4h6m-7-8h8a2 2 0 012 2v10a2 2 0 01-2 2H7a2 2 0 01-2-2V10a2 2 0 012-2z"
              />
            </svg>
            <span class="text-sm font-semibold text-primary">API Docs</span>
          </div>
          <p class="text-sm text-muted">
            Browse extracted API docs from published wheels — no external docs site needed.
          </p>
        </div>
      </div>
    </section>

    <section class="pb-16">
      <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">Trending</h2>

      <!-- Skeleton loading state -->
      <div v-if="status === 'pending'" class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="i in POPULAR_LIMIT"
          :key="i"
          class="h-20 animate-pulse rounded-lg border border-subtle bg-raised/50"
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
