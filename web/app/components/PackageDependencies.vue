<script setup lang="ts">
import type { DependencyTree } from "~/types/api";

const props = defineProps<{
  name: string;
  dependencies: DependencyTree;
}>();

const activeExtra = ref<string | null>(null);

const extraNames = computed(() => Object.keys(props.dependencies.extras));

function toggleExtra(extra: string) {
  activeExtra.value = activeExtra.value === extra ? null : extra;
}
</script>

<template>
  <div class="space-y-6">
    <!-- Required dependencies -->
    <div>
      <h2 class="mb-3 text-xs font-medium uppercase tracking-wider text-zinc-500">
        Required ({{ dependencies.required.length }})
      </h2>
      <div v-if="dependencies.required.length" class="space-y-1.5">
        <div
          v-for="dep in dependencies.required"
          :key="dep.name"
          class="flex items-center justify-between rounded border border-zinc-800 bg-zinc-900/50 px-4 py-2"
        >
          <NuxtLink
            :to="`/packages/${dep.name}`"
            class="font-mono text-sm text-zinc-50 hover:text-[var(--color-brand)]"
          >
            {{ dep.name }}
          </NuxtLink>
          <span class="font-mono text-xs text-zinc-500">{{ dep.constraint }}</span>
        </div>
      </div>
      <p v-else class="text-sm text-zinc-500">No required dependencies.</p>
    </div>

    <!-- Extras -->
    <div v-if="extraNames.length">
      <h2 class="mb-3 text-xs font-medium uppercase tracking-wider text-zinc-500">Extras</h2>
      <div class="mb-3 flex flex-wrap gap-2">
        <button
          v-for="extra in extraNames"
          :key="extra"
          class="rounded px-3 py-1 font-mono text-xs transition-colors"
          :class="
            activeExtra === extra
              ? 'bg-[var(--color-brand-border)] text-[var(--color-brand)]'
              : 'bg-zinc-800 text-zinc-400 hover:text-zinc-200'
          "
          @click="toggleExtra(extra)"
        >
          {{ extra }}
        </button>
      </div>
      <div v-if="activeExtra" class="space-y-1.5">
        <div
          v-for="dep in dependencies.extras[activeExtra]"
          :key="dep.name"
          class="flex items-center justify-between rounded border border-zinc-800 bg-zinc-900/50 px-4 py-2"
        >
          <NuxtLink
            :to="`/packages/${dep.name}`"
            class="font-mono text-sm text-zinc-50 hover:text-[var(--color-brand)]"
          >
            {{ dep.name }}
          </NuxtLink>
          <span class="font-mono text-xs text-zinc-500">{{ dep.constraint }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
