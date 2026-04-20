<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, reset } =
  useSearchTypeahead();
const inputRef = ref<HTMLInputElement | null>(null);
const isModalOpen = ref(false);
const colorMode = useColorMode();
const { withTransition } = useThemeTransition();

function openModal() {
  isModalOpen.value = true;
  reset();
  nextTick(() => inputRef.value?.focus());
}

function closeModal() {
  isModalOpen.value = false;
  reset();
}

function setTheme(mode: string) {
  withTransition(() => {
    colorMode.preference = mode;
  });
  closeModal();
}

function onModalKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    closeModal();
    return;
  }
  onKeydown(e);
}

const route = useRoute();

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    if (route.path === "/") {
      const heroInput = document.querySelector<HTMLInputElement>('main input[type="text"]');
      if (heroInput) {
        heroInput.focus();
        return;
      }
    }
    if (isModalOpen.value) {
      closeModal();
    } else {
      openModal();
    }
  }
}

onMounted(() => {
  window.addEventListener("keydown", onGlobalKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", onGlobalKeydown);
});
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-100 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="isModalOpen"
        class="fixed inset-0 z-[100] flex items-start justify-center bg-black/60 pt-[20vh]"
        @mousedown.self="closeModal"
      >
        <div
          class="w-full max-w-lg overflow-hidden rounded-xl border border-[var(--color-brand-border)] bg-surface shadow-2xl"
        >
          <!-- Input area -->
          <div class="flex items-center gap-3 border-b border-subtle px-4 py-3">
            <svg
              class="size-4 shrink-0 text-muted"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke-width="1.5"
              stroke="currentColor"
              aria-hidden="true"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
              />
            </svg>
            <input
              ref="inputRef"
              v-model="query"
              type="text"
              placeholder="Search packages..."
              aria-label="Search Python packages"
              class="min-w-0 flex-1 bg-transparent text-sm text-primary placeholder-muted outline-none"
              @keydown="onModalKeydown"
            />
            <kbd
              class="hidden shrink-0 rounded border border-subtle px-1.5 py-0.5 text-xs text-muted sm:block"
            >
              ESC
            </kbd>
          </div>

          <!-- Search results -->
          <SearchDropdown
            :results="results"
            :selected-index="selectedIndex"
            :loading="isLoading"
            :has-query="!!query.trim()"
            class="border-0 rounded-none shadow-none"
            @select="
              (r) => {
                navigateToResult(r);
                closeModal();
              }
            "
            @hover="(i) => (selectedIndex = i)"
          />

          <!-- Theme section — shown when no search query -->
          <div v-if="!query.trim()" class="border-t border-subtle px-2 py-2">
            <p
              class="px-2 pb-1 pt-0.5 text-[10px] font-semibold uppercase tracking-widest text-muted"
            >
              Theme
            </p>
            <button
              v-for="item in [
                { mode: 'light', label: 'Light mode' },
                { mode: 'dark', label: 'Dark mode' },
                { mode: 'system', label: 'System default' },
              ]"
              :key="item.mode"
              type="button"
              class="flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-sm text-primary transition-colors hover:bg-raised"
              @click="setTheme(item.mode)"
            >
              <!-- Sun icon -->
              <svg
                v-if="item.mode === 'light'"
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                class="text-muted"
                aria-hidden="true"
              >
                <circle cx="12" cy="12" r="5" />
                <line x1="12" y1="1" x2="12" y2="3" />
                <line x1="12" y1="21" x2="12" y2="23" />
                <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                <line x1="1" y1="12" x2="3" y2="12" />
                <line x1="21" y1="12" x2="23" y2="12" />
                <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
              </svg>
              <!-- Moon icon -->
              <svg
                v-else-if="item.mode === 'dark'"
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                class="text-muted"
                aria-hidden="true"
              >
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </svg>
              <!-- Monitor icon -->
              <svg
                v-else
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
                class="text-muted"
                aria-hidden="true"
              >
                <rect x="2" y="3" width="20" height="14" rx="2" />
                <line x1="8" y1="21" x2="16" y2="21" />
                <line x1="12" y1="17" x2="12" y2="21" />
              </svg>
              <span>{{ item.label }}</span>
              <span
                v-if="colorMode.preference === item.mode"
                class="ml-auto text-[var(--color-brand)]"
                >✓</span
              >
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
