<script setup lang="ts">
const searchQuery = ref('')
const router = useRouter()

function onSearch() {
  if (searchQuery.value.trim()) {
    router.push({ path: '/search', query: { q: searchQuery.value.trim() } })
  }
}
</script>

<template>
  <header class="sticky top-0 z-50 border-b border-zinc-800 bg-zinc-950/80 backdrop-blur-sm">
    <div class="mx-auto flex h-14 max-w-6xl items-center gap-6 px-4">
      <NuxtLink to="/" class="flex items-center gap-2 text-zinc-50 hover:text-white">
        <span class="text-lg font-bold tracking-tight">pypx</span>
      </NuxtLink>

      <form class="flex-1" @submit.prevent="onSearch">
        <div class="relative max-w-md">
          <UInput
            v-model="searchQuery"
            placeholder="Search packages..."
            icon="i-heroicons-magnifying-glass"
            size="sm"
          />
        </div>
      </form>

      <nav class="flex items-center gap-4">
        <UColorModeButton />
      </nav>
    </div>
  </header>
</template>
