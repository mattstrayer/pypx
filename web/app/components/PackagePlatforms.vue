<script setup lang="ts">
import type { PlatformCoverage } from "~/types/api";

const props = defineProps<{
  coverage: PlatformCoverage;
}>();

interface Platform {
  key: keyof PlatformCoverage;
  label: string;
  short: string;
}

const platforms: Platform[] = [
  { key: "pure_python", label: "Pure Python", short: "py" },
  { key: "linux_x86_64", label: "Linux x86_64", short: "linux" },
  { key: "linux_arm64", label: "Linux ARM64", short: "arm64" },
  { key: "macos_x86_64", label: "macOS Intel", short: "mac-x86" },
  { key: "macos_arm64", label: "macOS Apple Silicon", short: "mac-arm" },
  { key: "windows_x86_64", label: "Windows x86_64", short: "win" },
  { key: "musl", label: "musl/Alpine", short: "musl" },
];

const supported = computed(() =>
  platforms.filter((p) => p.key !== "pure_python" && props.coverage[p.key]),
);
const hasAnyCoverage = computed(() => supported.value.length > 0);
</script>

<template>
  <div v-if="hasAnyCoverage">
    <h2 class="mb-2.5 text-[10px] font-semibold uppercase tracking-[0.08em] text-muted">
      Platforms
    </h2>
    <div class="flex flex-wrap gap-1.5">
      <span
        v-for="p in supported"
        :key="p.key"
        :title="p.label"
        class="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono bg-zinc-100 text-zinc-600 ring-1 ring-zinc-200 dark:bg-neutral-800 dark:text-neutral-300 dark:ring-neutral-700"
      >
        {{ p.short }}
      </span>
    </div>
  </div>
</template>
