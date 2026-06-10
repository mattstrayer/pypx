<script setup lang="ts">
import type { CompareData } from "~/types/api";

const route = useRoute();
const router = useRouter();

// Initialize from URL query param, max 5 slots
const initialPkgs = computed(() => {
  const raw = route.query.pkgs as string | undefined;
  if (!raw) return ["", ""];
  const parts = raw
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .slice(0, 5);
  return parts.length >= 2 ? parts : [...parts, ...["", ""].slice(parts.length - 2)];
});

const pkgInputs = ref<string[]>(initialPkgs.value);

function addPackage() {
  if (pkgInputs.value.length < 5) {
    pkgInputs.value.push("");
  }
}

function removePackage(i: number) {
  if (pkgInputs.value.length > 2) {
    pkgInputs.value.splice(i, 1);
  }
}

const validNames = computed(() =>
  pkgInputs.value.map((s) => s.trim().toLowerCase()).filter(Boolean),
);

const canCompare = computed(() => validNames.value.length >= 2);

// The committed list of names used for the current fetch
const committedNames = ref<string[]>(
  (() => {
    const raw = route.query.pkgs as string | undefined;
    if (!raw) return [];
    return raw
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean)
      .slice(0, 5);
  })(),
);

function runCompare() {
  const names = validNames.value;
  if (names.length < 2) return;
  router.replace({ query: { pkgs: names.join(",") } });
  committedNames.value = names;
}

const api = useApi();

const {
  data: compareData,
  status,
  error,
} = useAsyncData<CompareData | null>(
  () => `compare-${committedNames.value.join(",")}`,
  async () => {
    if (committedNames.value.length < 2) return null;
    return api.fetchCompare(committedNames.value);
  },
  { watch: [committedNames] },
);

// Column order: successful packages first, then skipped
const packages = computed(() => compareData.value?.Packages ?? []);
const skipped = computed(() => compareData.value?.Skipped ?? []);

function formatInstallSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDownloads(n: number): string {
  if (!n) return "—";
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}K`;
  if (n < 1_000_000_000) return `${Math.floor(n / 1_000_000)}M`;
  return `${(n / 1_000_000_000).toFixed(1)}B`;
}

const pkgTitle = computed(() => {
  const names = packages.value.map((p) => p.Name);
  if (!names.length) return "Compare packages — pypx";
  return `Compare ${names.join(" vs ")} — pypx`;
});

useSeoMeta({
  title: pkgTitle,
  description: "Compare Python packages side by side — versions, downloads, deps, and more.",
  ogTitle: pkgTitle,
});
</script>

<template>
  <div>
    <div class="mb-8">
      <h1 class="text-2xl font-bold tracking-tight text-primary">Compare Packages</h1>
      <p class="mt-1 text-sm text-muted">Select up to 5 packages to compare side by side.</p>
    </div>

    <!-- Input form -->
    <div class="mb-6 rounded-[14px] border border-subtle bg-surface p-4">
      <div class="flex flex-wrap gap-2">
        <div v-for="(_, i) in pkgInputs" :key="i" class="flex items-center gap-1">
          <input
            v-model="pkgInputs[i]"
            type="text"
            :placeholder="`package ${i + 1}`"
            class="w-36 rounded-lg border border-subtle bg-raised px-3 py-1.5 text-sm text-primary placeholder-muted outline-none transition focus:border-[var(--color-brand-border)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
            @keydown.enter="runCompare"
          />
          <button
            v-if="pkgInputs.length > 2"
            type="button"
            class="rounded p-0.5 text-muted hover:text-primary"
            aria-label="Remove"
            @click="removePackage(i)"
          >
            ✕
          </button>
        </div>

        <button
          v-if="pkgInputs.length < 5"
          type="button"
          class="rounded-lg border border-dashed border-subtle px-3 py-1.5 text-xs text-muted transition hover:border-[var(--color-brand-border)] hover:text-[var(--color-brand)]"
          @click="addPackage"
        >
          + add
        </button>
      </div>

      <div class="mt-3 flex items-center gap-3">
        <button
          type="button"
          :disabled="!canCompare"
          class="rounded-lg bg-[var(--color-brand)] px-4 py-1.5 text-sm font-semibold text-white transition disabled:opacity-40 hover:opacity-90"
          @click="runCompare"
        >
          Compare
        </button>
        <span v-if="!canCompare && pkgInputs.some((s) => s.trim())" class="text-xs text-muted">
          Enter at least 2 package names.
        </span>
      </div>
    </div>

    <!-- Skipped packages warning -->
    <div
      v-if="skipped.length"
      class="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-sm dark:border-amber-900 dark:bg-amber-950/30"
    >
      <span class="font-medium text-amber-800 dark:text-amber-300">Could not load:</span>
      <span v-for="s in skipped" :key="s.Name" class="ml-2 text-amber-700 dark:text-amber-400">
        {{ s.Name }} ({{ s.Reason }})
      </span>
    </div>

    <!-- Loading skeleton -->
    <div
      v-if="status === 'pending'"
      class="overflow-hidden rounded-[14px] border border-subtle bg-surface"
    >
      <div
        v-for="i in 8"
        :key="i"
        class="h-10 animate-pulse border-b border-subtle bg-raised/30 last:border-b-0"
      />
    </div>

    <!-- Error state -->
    <div
      v-else-if="status === 'error'"
      class="rounded-[14px] border border-subtle bg-surface px-6 py-8 text-center text-sm text-muted"
    >
      Couldn't load one or more packages. Check the package names and try again.
    </div>

    <!-- Empty state (no query yet) -->
    <div
      v-else-if="!committedNames.length || !packages.length"
      class="rounded-[14px] border border-dashed border-subtle bg-surface px-6 py-12 text-center text-sm text-muted"
    >
      <p>Enter 2–5 package names above and press Compare.</p>
    </div>

    <!-- Compare table -->
    <CompareTable v-else :packages="packages" />
  </div>
</template>
