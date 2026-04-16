<script setup lang="ts">
const route = useRoute();
const isHomepage = computed(() => route.path === "/");
</script>

<template>
  <div class="min-h-screen bg-zinc-950 text-zinc-50">
    <AppHeader :hide-search="isHomepage" />
    <main class="mx-auto max-w-6xl px-4 py-8">
      <slot />
    </main>
  </div>
</template>
