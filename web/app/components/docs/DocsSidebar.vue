<script setup lang="ts">
import { DynamicScroller as RawDynamicScroller, DynamicScrollerItem } from "vue-virtual-scroller";
import "vue-virtual-scroller/dist/vue-virtual-scroller.css";
import type { DocSymbol } from "~/types/api";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const DynamicScroller = RawDynamicScroller as unknown as new (...args: any) => any;

const props = defineProps<{
  functions: DocSymbol[];
  classes: DocSymbol[];
  exceptions: DocSymbol[];
  activeSymbol: string | null;
}>();

const emit = defineEmits<{
  select: [name: string];
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
</script>

<template>
  <div
    class="hidden w-[216px] flex-shrink-0 flex-col border-r border-subtle bg-base md:sticky md:top-0 md:flex md:h-screen"
  >
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
            class="flex w-full items-center justify-between px-3 hover:bg-raised/50 transition-colors cursor-pointer"
            style="height: 36px"
            @click="toggleSection(item.kind)"
          >
            <span
              class="flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-primary"
            >
              <svg
                class="h-3 w-3 flex-shrink-0 text-[var(--color-brand)] transition-transform duration-150"
                :class="collapsed.has(item.kind) ? '-rotate-90' : 'rotate-0'"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2.5"
              >
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
              {{ item.label }}
            </span>
            <span
              class="rounded-full bg-[var(--color-brand-muted)] px-1.5 py-0.5 text-[9px] text-[var(--color-brand)]"
              >{{ item.count }}</span
            >
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
                : 'text-muted hover:bg-raised hover:text-primary'
            "
            :aria-current="activeSymbol === item.name ? 'location' : undefined"
            @click="emit('select', item.name)"
          >
            {{ item.name }}
          </button>
        </DynamicScrollerItem>
      </template>
    </DynamicScroller>
  </div>
</template>
