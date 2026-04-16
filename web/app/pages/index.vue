<script setup lang="ts">
const searchQuery = ref("");
const router = useRouter();

function onSearch() {
  if (searchQuery.value.trim()) {
    router.push({ path: "/search", query: { q: searchQuery.value.trim() } });
  }
}

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
      <p class="mt-3 max-w-lg text-lg text-zinc-400">
        The Python Package Index, reimagined. Fast search, dependency insights, and download trends
        — all in one place.
      </p>
      <form class="mt-8 w-full max-w-xl" @submit.prevent="onSearch">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search 500,000+ Python packages..."
          class="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3 text-zinc-50 placeholder-zinc-500 outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
        />
      </form>
    </section>

    <section class="pb-16">
      <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-zinc-500">
        Popular Packages
      </h2>

      <!-- Skeleton loading state -->
      <div v-if="status === 'pending'" class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="i in POPULAR_LIMIT"
          :key="i"
          class="h-20 animate-pulse rounded-lg border border-zinc-800 bg-zinc-800/50"
        />
      </div>

      <!-- Error state -->
      <p v-else-if="status === 'error'" class="text-sm text-zinc-500">
        Could not load popular packages.
      </p>

      <!-- Data -->
      <TrendingPackages v-else-if="popularPackages?.length" :packages="popularPackages" />
      <p v-else class="text-sm text-zinc-500">No popular packages available.</p>
    </section>
  </div>
</template>
