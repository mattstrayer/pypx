<script setup lang="ts">
import { DynamicScroller, DynamicScrollerItem } from "vue-virtual-scroller";
import "vue-virtual-scroller/dist/vue-virtual-scroller.css";
import type { DocSymbol } from "~/types/api";

const props = defineProps<{
  functions: DocSymbol[];
  classes: DocSymbol[];
  exceptions: DocSymbol[];
  activeSymbol: string | null;
}>();

const emit = defineEmits<{
  select: [name: string];
  "open-palette": [];
}>();

type SidebarItem =
  | { id: string; type: "header"; label: string; kind: string; count: number }
  | { id: string; type: "symbol"; name: string; kind: string };

const collapsed = ref(new Set<string>());

function toggleSection(kind: string) {
  const next = new Set(collapsed.value);
  if (next.has(kind)) next.delete(kind);
  else next.add(kind);
  collapsed.value = next;
}

const items = computed<SidebarItem[]>(() => {
  const list: SidebarItem[] = [];
  if (props.functions.length) {
    list.push({
      id: "h-functions",
      type: "header",
      label: "Functions",
      kind: "functions",
      count: props.functions.length,
    });
    if (!collapsed.value.has("functions")) {
      for (const s of props.functions)
        list.push({ id: `s-${s.name}`, type: "symbol", name: s.name, kind: s.kind });
    }
  }
  if (props.classes.length) {
    list.push({
      id: "h-classes",
      type: "header",
      label: "Classes",
      kind: "classes",
      count: props.classes.length,
    });
    if (!collapsed.value.has("classes")) {
      for (const s of props.classes)
        list.push({ id: `s-${s.name}`, type: "symbol", name: s.name, kind: s.kind });
    }
  }
  if (props.exceptions.length) {
    list.push({
      id: "h-exceptions",
      type: "header",
      label: "Exceptions",
      kind: "exceptions",
      count: props.exceptions.length,
    });
    if (!collapsed.value.has("exceptions")) {
      for (const s of props.exceptions)
        list.push({ id: `s-${s.name}`, type: "symbol", name: s.name, kind: s.kind });
    }
  }
  return list;
});

const activeIndex = computed(() =>
  items.value.findIndex((i) => i.type === "symbol" && i.name === props.activeSymbol),
);

const scrollerRef = ref<InstanceType<typeof DynamicScroller> | null>(null);

watch(activeIndex, (idx) => {
  if (idx >= 0 && scrollerRef.value?.scrollToItem) {
    scrollerRef.value.scrollToItem(idx);
  }
});

const shortcutLabel = ref("⌘K");
onMounted(() => {
  shortcutLabel.value = /mac/i.test(navigator.userAgent) ? "⌘K" : "Ctrl+K";
});
</script>

<template>
  <div
    class="w-48 flex-shrink-0 sticky top-0 h-screen flex flex-col border-r border-subtle bg-base hidden md:flex"
  >
    <!-- ⌘K trigger -->
    <div class="px-2 py-2 border-b border-subtle flex-shrink-0">
      <button
        data-testid="palette-trigger"
        class="flex w-full items-center gap-2 rounded-md bg-zinc-800/50 px-2.5 py-1.5 text-[11px] text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800 transition-colors"
        @click="emit('open-palette')"
      >
        <svg
          class="h-3 w-3 flex-shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2.5"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
        <span class="flex-1 text-left">Jump to symbol</span>
        <kbd class="text-[9px] text-zinc-600">{{ shortcutLabel }}</kbd>
      </button>
    </div>

    <!-- Virtual symbol list -->
    <DynamicScroller
      ref="scrollerRef"
      :items="items"
      :min-item-size="28"
      key-field="id"
      class="flex-1 overflow-y-auto py-1"
    >
      <template #default="{ item, index, active }">
        <DynamicScrollerItem :item="item" :active="active" :data-index="index">
          <!-- Section header -->
          <button
            v-if="item.type === 'header'"
            data-testid="section-header"
            class="flex w-full items-center justify-between px-3 pb-1 pt-3 hover:bg-zinc-800/30 transition-colors cursor-pointer"
            style="height: 36px"
            @click="toggleSection(item.kind)"
          >
            <span
              class="flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-wide text-zinc-500"
            >
              <span class="text-[8px]">{{ collapsed.has(item.kind) ? "▸" : "▾" }}</span>
              {{ item.label }}
            </span>
            <span class="rounded-full bg-zinc-800 px-1.5 py-0.5 text-[9px] text-zinc-600">{{
              item.count
            }}</span>
          </button>

          <!-- Symbol row -->
          <button
            v-else
            data-testid="symbol-row"
            class="flex w-full items-center px-3 text-left font-mono text-[12px] transition-colors"
            style="height: 28px"
            :class="
              activeSymbol === item.name
                ? 'bg-[var(--color-brand-muted)] border-l-2 border-[var(--color-brand)] text-primary pl-2.5'
                : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200'
            "
            @click="emit('select', item.name)"
          >
            {{ item.name }}
          </button>
        </DynamicScrollerItem>
      </template>
    </DynamicScroller>
  </div>
</template>
