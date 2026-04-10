<script setup lang="ts">
const searchQuery = ref("");
const router = useRouter();
const colorMode = useColorMode();

function onSearch() {
  if (searchQuery.value.trim()) {
    router.push({ path: "/search", query: { q: searchQuery.value.trim() } });
  }
}

function toggleColorMode() {
  colorMode.preference = colorMode.value === "dark" ? "light" : "dark";
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
          <svg
            class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-zinc-500"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.3-4.3" />
          </svg>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search packages..."
            class="w-full rounded-md border border-zinc-800 bg-zinc-900 py-1.5 pl-8 pr-2.5 text-sm text-zinc-50 placeholder-zinc-500 outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600"
          />
        </div>
      </form>

      <nav class="flex items-center gap-4">
        <button
          type="button"
          :aria-label="colorMode.value === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'"
          class="rounded-md p-1.5 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-50"
          @click="toggleColorMode"
        >
          <svg
            v-if="colorMode.value === 'dark'"
            class="size-5"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
          </svg>
          <svg
            v-else
            class="size-5"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <circle cx="12" cy="12" r="4" />
            <path
              d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32 1.41 1.41M2 12h2m16 0h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"
            />
          </svg>
        </button>
      </nav>
    </div>
  </header>
</template>
