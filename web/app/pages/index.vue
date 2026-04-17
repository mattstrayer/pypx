<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, close } =
  useSearchTypeahead();
const searchWrapper = ref<HTMLElement | null>(null);

function onSearch() {
  if (query.value.trim()) {
    navigateTo({ path: "/search", query: { q: query.value.trim() } });
    close();
  }
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
const { data: popularPackages, status } = await useAsyncData("popular", () =>
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
    </section>

    <section class="pb-16">
      <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-muted">Popular Packages</h2>

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
      <TrendingPackages v-else-if="popularPackages?.length" :packages="popularPackages" />
      <p v-else class="text-sm text-muted">No popular packages available.</p>
    </section>
  </div>
</template>
