<script setup lang="ts">
const searchQuery = ref('')
const router = useRouter()

function onSearch() {
  if (searchQuery.value.trim()) {
    router.push({ path: '/search', query: { q: searchQuery.value.trim() } })
  }
}

const trendingPackages = ref([
  { name: 'requests', summary: 'HTTP for Humans', downloads: 50000000 },
  { name: 'flask', summary: 'A simple framework for building complex web applications', downloads: 30000000 },
  { name: 'django', summary: 'The web framework for perfectionists with deadlines', downloads: 25000000 },
  { name: 'numpy', summary: 'Fundamental package for scientific computing', downloads: 80000000 },
  { name: 'pandas', summary: 'Powerful data structures for data analysis', downloads: 60000000 },
  { name: 'fastapi', summary: 'Modern, fast web framework for building APIs', downloads: 20000000 },
])

useSeoMeta({
  title: 'pypx — A modern PyPI frontend',
  description: 'Explore Python packages with enriched insights, fast search, dependency analysis, and download trends.',
  ogTitle: 'pypx — A modern PyPI frontend',
  ogDescription: 'Explore Python packages with enriched insights, fast search, dependency analysis, and download trends.',
})
</script>

<template>
  <div>
    <section class="flex flex-col items-center pt-16 pb-12 text-center">
      <h1 class="text-5xl font-bold tracking-tight text-zinc-50">pypx</h1>
      <p class="mt-3 max-w-lg text-lg text-zinc-400">
        The Python Package Index, reimagined. Fast search, dependency insights, and download trends — all in one place.
      </p>
      <form class="mt-8 w-full max-w-xl" @submit.prevent="onSearch">
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search 500,000+ Python packages..."
          class="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3 text-zinc-50 placeholder-zinc-500 outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600"
        />
      </form>
    </section>

    <section class="pb-16">
      <h2 class="mb-4 text-sm font-medium uppercase tracking-wider text-zinc-500">Popular Packages</h2>
      <TrendingPackages :packages="trendingPackages" />
    </section>
  </div>
</template>
