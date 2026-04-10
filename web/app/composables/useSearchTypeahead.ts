import { useDebounceFn } from "@vueuse/core";
import type { SearchResult } from "~/types/api";

export function useSearchTypeahead() {
  const query = ref("");
  const results = ref<SearchResult[]>([]);
  const selectedIndex = ref(-1);
  const isOpen = ref(false);
  const isLoading = ref(false);
  const router = useRouter();
  const { searchPackages } = useApi();

  const performSearch = useDebounceFn(async (q: string) => {
    if (!q.trim()) {
      results.value = [];
      isLoading.value = false;
      return;
    }
    try {
      results.value = await searchPackages(q, 5);
      selectedIndex.value = -1;
    } catch {
      results.value = [];
    } finally {
      isLoading.value = false;
    }
  }, 150);

  watch(query, (val) => {
    if (val.trim()) {
      isLoading.value = true;
      isOpen.value = true;
    } else {
      isOpen.value = false;
    }
    performSearch(val);
  });

  function onKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      close();
      return;
    }
    if (!isOpen.value || results.value.length === 0) {
      if (e.key === "Enter") {
        e.preventDefault();
        navigateToSearch();
      }
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      selectedIndex.value = Math.min(selectedIndex.value + 1, results.value.length - 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      selectedIndex.value = Math.max(selectedIndex.value - 1, -1);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const selected = results.value[selectedIndex.value];
      if (selectedIndex.value >= 0 && selected) {
        navigateToResult(selected);
      } else {
        navigateToSearch();
      }
    }
  }

  function navigateToResult(result: SearchResult) {
    router.push(`/packages/${result.name}`);
    close();
  }

  function navigateToSearch() {
    if (query.value.trim()) {
      router.push({ path: "/search", query: { q: query.value.trim() } });
      close();
    }
  }

  function open() {
    isOpen.value = true;
    selectedIndex.value = -1;
  }

  function close() {
    isOpen.value = false;
    selectedIndex.value = -1;
  }

  function reset() {
    query.value = "";
    results.value = [];
    selectedIndex.value = -1;
    isOpen.value = false;
    isLoading.value = false;
  }

  return {
    query,
    results,
    selectedIndex,
    isOpen,
    isLoading,
    onKeydown,
    navigateToResult,
    open,
    close,
    reset,
  };
}
