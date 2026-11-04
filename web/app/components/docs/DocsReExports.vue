<script setup lang="ts">
import type { DocModule } from "~/types/api";

defineProps<{
  modules: DocModule[];
}>();
</script>

<template>
  <section id="re-exports" data-testid="re-exports" class="mt-8 scroll-mt-16">
    <h2 class="mb-3 text-sm font-semibold text-primary">Re-exports</h2>
    <div
      v-for="mod in modules"
      :key="mod.name"
      class="mb-4 rounded-xl border border-subtle bg-surface p-5"
    >
      <h3 class="mb-2 font-mono text-[11px] font-normal text-muted [overflow-wrap:anywhere]">
        {{ mod.name }}
      </h3>
      <ul class="space-y-1">
        <li
          v-for="alias in mod.aliases"
          :key="alias.name"
          class="font-mono text-[12px] text-muted [overflow-wrap:anywhere]"
        >
          <span class="text-primary">{{ alias.name }}</span> -&gt; {{ alias.target }}
        </li>
      </ul>
    </div>
  </section>
</template>
