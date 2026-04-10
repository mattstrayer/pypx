<script setup lang="ts">
import { useDebounceFn } from "@vueuse/core";
import type { SearchResult } from "~/types/api";

const isOpen = ref(false);
const query = ref("");
const results = ref<SearchResult[]>([]);
const selectedIndex = ref(0);
const isLoading = ref(false);
const router = useRouter();
const { searchPackages } = useApi();

function open() {
  isOpen.value = true;
  query.value = "";
  results.value = [];
  selectedIndex.value = 0;
}

function close() {
  isOpen.value = false;
}

const performSearch = useDebounceFn(async (q: string) => {
  if (!q.trim()) {
    results.value = [];
    isLoading.value = false;
    return;
  }
  try {
    results.value = await searchPackages(q);
    selectedIndex.value = 0;
  } catch {
    results.value = [];
  } finally {
    isLoading.value = false;
  }
}, 150);

watch(query, (val) => {
  if (val.trim()) {
    isLoading.value = true;
  }
  performSearch(val);
});

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    close();
    return;
  }
  if (e.key === "ArrowDown") {
    e.preventDefault();
    selectedIndex.value = Math.min(selectedIndex.value + 1, results.value.length - 1);
  } else if (e.key === "ArrowUp") {
    e.preventDefault();
    selectedIndex.value = Math.max(selectedIndex.value - 1, 0);
  } else if (e.key === "Enter" && results.value.length > 0) {
    navigateToResult(results.value[selectedIndex.value]);
  }
}

function navigateToResult(result: SearchResult) {
  router.push(`/packages/${result.name}`);
  close();
}

function onGlobalKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === "k") {
    e.preventDefault();
    if (isOpen.value) {
      close();
    } else {
      open();
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
        v-if="isOpen"
        class="fixed inset-0 z-[100] flex items-start justify-center bg-black/60 pt-[20vh]"
        @mousedown.self="close"
      >
        <div
          class="w-full max-w-lg rounded-xl border border-zinc-700 bg-zinc-900 shadow-2xl"
          @keydown="onKeydown"
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
              v-model="query"
              autofocus
              type="text"
              placeholder="Search packages..."
              class="min-w-0 flex-1 bg-transparent text-sm text-zinc-100 placeholder-zinc-500 outline-none"
            />
            <kbd
              class="hidden shrink-0 rounded border border-zinc-700 px-1.5 py-0.5 text-xs text-zinc-500 sm:block"
              >ESC</kbd
            >
          </div>

          <!-- Results -->
          <div v-if="query.trim()" class="max-h-80 overflow-y-auto">
            <template v-if="results.length > 0">
              <button
                v-for="(result, index) in results"
                :key="result.name"
                class="flex w-full flex-col gap-0.5 px-4 py-2.5 text-left transition-colors"
                :class="
                  index === selectedIndex
                    ? 'bg-zinc-800 text-zinc-50'
                    : 'text-zinc-400 hover:bg-zinc-800/50'
                "
                @click="navigateToResult(result)"
                @mousemove="selectedIndex = index"
              >
                <span
                  class="text-sm font-medium"
                  :class="index === selectedIndex ? 'text-zinc-50' : 'text-zinc-200'"
                  >{{ result.name }}</span
                >
                <span
                  v-if="result.summary"
                  class="truncate text-xs"
                  :class="index === selectedIndex ? 'text-zinc-400' : 'text-zinc-500'"
                  >{{ result.summary }}</span
                >
              </button>
            </template>
            <div v-else-if="!isLoading" class="px-4 py-6 text-center text-sm text-zinc-500">
              No packages found
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
