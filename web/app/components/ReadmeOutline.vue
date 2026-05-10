<script setup lang="ts">
import { useThrottleFn } from "@vueuse/core";

interface OutlineItem {
  id: string;
  text: string;
  level: number;
}

const props = defineProps<{
  containerSelector: string;
}>();

const items = ref<OutlineItem[]>([]);
const activeId = ref("");

function buildOutline() {
  const container = document.querySelector(props.containerSelector);
  if (!container) return;

  const headings = container.querySelectorAll("h1, h2, h3, h4");
  const outline: OutlineItem[] = [];

  headings.forEach((heading, index) => {
    const el = heading as HTMLElement;
    if (!el.id) {
      el.id = `readme-heading-${index}`;
    }
    outline.push({
      id: el.id,
      text: el.textContent?.trim() || "",
      level: parseInt(el.tagName[1] ?? "2"),
    });
  });

  items.value = outline;
}

function scrollTo(id: string) {
  const el = document.getElementById(id);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "start" });
    activeId.value = id;
  }
}

function onScroll() {
  const headings = items.value
    .map((item) => ({
      id: item.id,
      top: document.getElementById(item.id)?.getBoundingClientRect().top ?? Infinity,
    }))
    .filter((h) => h.top <= 120);

  if (headings.length > 0) {
    activeId.value = headings[headings.length - 1]!.id;
  }
}

const throttledScroll = useThrottleFn(onScroll, 100);

let observer: MutationObserver | null = null;

onMounted(() => {
  nextTick(() => {
    buildOutline();
    window.addEventListener("scroll", throttledScroll, { passive: true });

    // Watch for content changes (e.g., package navigation).
    const container = document.querySelector(props.containerSelector);
    if (container) {
      observer = new MutationObserver(() => {
        buildOutline();
      });
      observer.observe(container, { childList: true, subtree: true });
    }
  });
});

onUnmounted(() => {
  window.removeEventListener("scroll", throttledScroll);
  observer?.disconnect();
});
</script>

<template>
  <nav v-if="items.length >= 3" class="hidden xl:block">
    <h3 class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">On this page</h3>
    <ul class="space-y-1 border-l border-subtle">
      <li v-for="item in items" :key="item.id">
        <button
          class="block w-full truncate border-l-2 py-0.5 text-left text-xs transition-colors"
          :class="[
            activeId === item.id
              ? 'border-[var(--color-brand)] text-zinc-800 dark:text-zinc-200'
              : 'border-transparent text-muted hover:text-zinc-700 dark:hover:text-zinc-300',
            item.level <= 2 ? 'pl-3' : 'pl-5',
          ]"
          @click="scrollTo(item.id)"
        >
          {{ item.text }}
        </button>
      </li>
    </ul>
  </nav>
</template>
