<script setup lang="ts">
const { query, results, selectedIndex, isOpen, isLoading, onKeydown, navigateToResult, reset } =
  useSearchTypeahead();
const inputRef = ref<HTMLInputElement | null>(null);
const isModalOpen = ref(false);

function openModal() {
  isModalOpen.value = true;
  reset();
  nextTick(() => inputRef.value?.focus());
}

function closeModal() {
  isModalOpen.value = false;
  reset();
}

function onModalKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    e.preventDefault();
    closeModal();
    return;
  }
  onKeydown(e);
}

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
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
          class="w-full max-w-lg overflow-hidden rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl"
        >
          <!-- Input area -->
          <div class="flex items-center gap-3 border-b border-zinc-800 px-4 py-3">
            <svg
              class="size-4 shrink-0 text-zinc-400"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              stroke-width="1.5"
              stroke="currentColor"
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
              class="min-w-0 flex-1 bg-transparent text-sm text-zinc-100 placeholder-zinc-500 outline-none"
              @keydown="onModalKeydown"
            />
            <kbd
              class="hidden shrink-0 rounded border border-zinc-700 px-1.5 py-0.5 text-xs text-zinc-500 sm:block"
            >
              ESC
            </kbd>
          </div>

          <!-- Results via shared dropdown (inline, not absolutely positioned) -->
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
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
