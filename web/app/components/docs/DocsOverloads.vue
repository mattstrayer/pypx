<script setup lang="ts">
defineProps<{
  overloads: string[];
}>();

const open = ref(false);
</script>

<template>
  <div data-testid="symbol-overloads">
    <button
      data-testid="symbol-overloads-toggle"
      class="flex cursor-pointer items-center gap-1.5 rounded-md py-0.5 text-[11px] text-muted transition-colors hover:text-primary"
      :aria-expanded="open"
      @click="open = !open"
    >
      <svg
        class="h-3 w-3 text-[var(--color-brand)] transition-transform duration-150"
        :class="open ? 'rotate-0' : '-rotate-90'"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2.5"
      >
        <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
      </svg>
      {{ overloads.length === 1 ? "1 overload" : `${overloads.length} overloads` }}
    </button>
    <ul
      v-if="open"
      class="mt-1.5 space-y-1 rounded-md border border-subtle bg-surface px-4 py-2.5 font-mono text-[11px] leading-relaxed text-primary"
    >
      <li
        v-for="(signature, i) in overloads"
        :key="i"
        data-testid="symbol-overload"
        class="[overflow-wrap:anywhere]"
      >
        {{ signature }}
      </li>
    </ul>
  </div>
</template>
