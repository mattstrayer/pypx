<script setup lang="ts">
import type { DocSymbol } from "~/types/api";

defineProps<{
  title: string;
  members: DocSymbol[];
}>();

const expanded = ref(false);
</script>

<template>
  <div class="mb-1">
    <button
      class="flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-[0.07em] text-muted transition-colors hover:bg-raised hover:text-primary"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <svg
        class="h-3 w-3 text-[var(--color-brand)] transition-transform duration-150"
        :class="expanded ? 'rotate-0' : '-rotate-90'"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2.5"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
      </svg>
      {{ title }}
      <span class="rounded-full bg-raised px-2 py-0.5 text-[9px] text-muted ring-1 ring-subtle">
        {{ members.length }}
      </span>
    </button>

    <div v-if="expanded" class="mt-3 space-y-5 border-l-2 border-subtle pl-4">
      <DocsMemberEntry v-for="member in members" :key="member.name" :member="member" />
    </div>
  </div>
</template>
