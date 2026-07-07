<script setup lang="ts">
import type { NuxtError } from "#app";

const props = defineProps<{ error: NuxtError }>();

const is404 = computed(() => props.error.statusCode === 404);

function goHome() {
  clearError({ redirect: "/" });
}
</script>

<template>
  <NuxtLayout>
    <div class="py-24 text-center">
      <p class="font-mono text-5xl font-bold text-primary">{{ error.statusCode }}</p>
      <p class="mt-4 text-lg font-medium text-zinc-700 dark:text-zinc-300">
        {{ is404 ? error.statusMessage || "Page not found" : "Something went wrong" }}
      </p>
      <p class="mt-1 text-sm text-muted">
        {{
          is404
            ? "Try searching for a package using the search bar above."
            : "The upstream service may be unavailable. Please try again shortly."
        }}
      </p>
      <button
        class="mt-6 cursor-pointer rounded-md border border-subtle bg-raised px-4 py-2 text-sm font-medium text-primary transition-colors hover:border-[var(--color-brand)]"
        @click="goHome"
      >
        Back to home
      </button>
    </div>
  </NuxtLayout>
</template>
