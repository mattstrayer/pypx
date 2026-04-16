<script setup lang="ts">
const route = useRoute();
const router = useRouter();

const query = computed(() => (route.query.q as string) ?? "");
const searchInput = ref(query.value);

watch(query, (val) => {
  searchInput.value = val;
});

function onSearch() {
  const q = searchInput.value.trim();
  if (q) {
    router.push({ path: "/search", query: { q } });
  }
}

const { searchPackages } = useApi();
const { data: results, status } = await useAsyncData("search", () => searchPackages(query.value), {
  watch: [query],
});

useSeoMeta({
  title: () => (query.value ? `"${query.value}"` : "Search"),
  description: "Search 500,000+ Python packages on pypx.",
});

defineOgImage("SiteCard", {}, { width: 1200, height: 630 });
</script>

<template>
  <div>
    <!-- Search input -->
    <form class="mb-6" @submit.prevent="onSearch">
      <input
        v-model="searchInput"
        type="text"
        placeholder="Search Python packages..."
        class="w-full rounded-lg border border-subtle bg-surface px-4 py-3 text-primary placeholder-muted outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
      />
    </form>

    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-subtle border-t-primary" />
    </div>

    <!-- Results -->
    <template v-else-if="results">
      <p class="mb-4 text-sm text-muted">
        {{ results.length }} {{ results.length === 1 ? "package" : "packages" }} found
        <span v-if="query">
          for <span class="text-zinc-700 dark:text-zinc-300">"{{ query }}"</span></span
        >
      </p>

      <SearchResults v-if="results.length > 0" :results="results" />

      <div v-else class="py-24 text-center">
        <p class="text-lg font-medium text-zinc-700 dark:text-zinc-300">No packages found</p>
        <p class="mt-1 text-sm text-muted">
          No results for <span class="text-muted">"{{ query }}"</span>. Try a different search term.
        </p>
      </div>
    </template>
  </div>
</template>
