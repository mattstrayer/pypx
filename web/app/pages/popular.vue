<script setup lang="ts">
const api = useApi();
const { data: packages, status } = await useAsyncData("popular-full", () => api.fetchPopular(50));

useHead({ titleTemplate: "%s" });
useSeoMeta({
  title: "Popular Python Packages — pypx",
  description: "The most-downloaded Python packages on PyPI, with download trends and summaries.",
  ogTitle: "Popular Python Packages — pypx",
  ogDescription: "The most-downloaded Python packages on PyPI, with download trends and summaries.",
});

defineOgImage("SiteCard", {}, { width: 1200, height: 630 });
</script>

<template>
  <div>
    <section class="pt-12 pb-8">
      <h1
        class="text-[clamp(1.75rem,4vw,2.5rem)] font-bold tracking-[-0.03em] text-primary leading-[1.1]"
      >
        Popular Packages
      </h1>
      <p class="mt-2 text-base text-muted leading-relaxed">
        The most-downloaded Python packages on PyPI, ranked by monthly installs.
      </p>
    </section>

    <section class="pb-16">
      <div class="mb-4 flex items-center gap-3">
        <span class="text-xs font-semibold uppercase tracking-[0.07em] text-muted">Top 50</span>
        <div class="h-px flex-1 bg-subtle" />
        <span class="font-mono text-[11.5px] text-muted opacity-70">
          by downloads · data from
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
        class="overflow-hidden rounded-[14px] border border-subtle bg-surface"
      >
        <div
          v-for="i in 50"
          :key="i"
          class="h-[52px] animate-pulse border-b border-subtle bg-raised/30 last:border-b-0"
        />
      </div>

      <!-- Error state -->
      <p v-else-if="status === 'error'" class="text-sm text-muted">
        Could not load popular packages.
      </p>

      <!-- Data -->
      <TrendingPackages v-else-if="packages?.length" :packages="packages" />
      <p v-else class="text-sm text-muted">No popular packages available.</p>
    </section>
  </div>
</template>
