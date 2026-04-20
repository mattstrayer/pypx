<script setup lang="ts">
defineProps<{
  hideSearch?: boolean;
}>();

const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, close } =
  useSearchTypeahead();
const searchWrapper = ref<HTMLElement | null>(null);

// Close dropdown when clicking outside the search wrapper
function onClickOutside(e: MouseEvent) {
  if (searchWrapper.value && !searchWrapper.value.contains(e.target as Node)) {
    close();
  }
}

function onSubmit() {
  const target = results.value[selectedIndex.value] ?? results.value[0];
  if (target) {
    navigateToResult(target);
    close();
  }
}

onMounted(() => {
  document.addEventListener("mousedown", onClickOutside);
});

onUnmounted(() => {
  document.removeEventListener("mousedown", onClickOutside);
});
</script>

<template>
  <header class="sticky top-0 z-50 border-b border-subtle bg-base/80 backdrop-blur-sm">
    <div class="mx-auto flex h-14 max-w-6xl items-center gap-6 px-4">
      <NuxtLink to="/" class="flex items-center gap-2">
        <div
          class="flex h-[26px] w-[26px] shrink-0 items-center justify-center rounded-[6px] border border-[var(--color-brand-border)] bg-[var(--color-brand-muted)] font-mono text-[9px] font-semibold tracking-[-0.04em] text-[var(--color-brand)]"
        >
          px
        </div>
        <span
          class="text-lg font-bold tracking-tight text-[var(--color-brand)] hover:text-[var(--color-brand-light)] transition-colors"
          >pypx</span
        >
      </NuxtLink>

      <div v-if="!hideSearch" ref="searchWrapper" class="relative flex-1 max-w-md">
        <form @submit.prevent="onSubmit">
          <div class="relative">
            <svg
              class="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted"
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
              v-model="query"
              type="text"
              placeholder="Search packages..."
              aria-label="Search Python packages"
              class="w-full rounded-md border border-subtle bg-surface py-1.5 pl-8 pr-12 text-sm text-primary placeholder-muted outline-none focus:border-[var(--color-brand-light)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
              @keydown="onKeydown"
              @focus="query.trim() && (isOpen = true)"
            />
            <kbd
              class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 hidden rounded bg-raised px-1.5 py-0.5 font-mono text-[10px] text-muted sm:inline"
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

      <ClientOnly>
        <ThemeToggle class="ml-auto" />
        <template #fallback>
          <div class="ml-auto size-8" />
        </template>
      </ClientOnly>
    </div>
  </header>
</template>
