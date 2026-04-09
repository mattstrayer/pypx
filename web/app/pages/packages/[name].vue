<script setup lang="ts">
const route = useRoute()
const name = computed(() => route.params.name as string)
const activeTab = ref('overview')

const { fetchPackage } = useApi()
const { data: pkg, status } = await useAsyncData(
  `package-${name.value}`,
  () => fetchPackage(name.value),
)

const tabs = [
  { key: 'overview', label: 'Overview' },
  { key: 'dependencies', label: 'Dependencies' },
  { key: 'versions', label: 'Versions' },
  { key: 'stats', label: 'Stats' },
]

useHead({
  title: computed(() => pkg.value ? `${pkg.value.name} — pypx` : 'Loading — pypx'),
})
</script>

<template>
  <div>
    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-300">Package not found</p>
      <p class="mt-1 text-sm text-zinc-500">No package named "{{ name }}" could be found.</p>
    </div>

    <!-- Loaded state -->
    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex items-baseline gap-3">
          <h1 class="text-3xl font-bold text-zinc-50">{{ pkg.name }}</h1>
          <span class="rounded bg-zinc-800 px-2 py-0.5 font-mono text-sm text-zinc-400">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-zinc-400">{{ pkg.summary }}</p>
        <div class="mt-3">
          <PackageBadges :pkg="pkg" />
        </div>
      </div>

      <!-- Tabs -->
      <div class="mb-6 flex gap-1 border-b border-zinc-800 pb-0">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="rounded-t px-4 py-2 text-sm font-medium transition-colors"
          :class="activeTab === tab.key
            ? 'bg-zinc-800 text-zinc-50'
            : 'text-zinc-500 hover:text-zinc-300'"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Tab content -->
      <div>
        <div v-if="activeTab === 'overview'">Overview content here</div>
        <div v-else-if="activeTab === 'dependencies'">Dependencies content here</div>
        <div v-else-if="activeTab === 'versions'">Versions content here</div>
        <div v-else-if="activeTab === 'stats'">Stats content here</div>
      </div>
    </div>
  </div>
</template>
