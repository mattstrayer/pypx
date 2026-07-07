<script setup lang="ts">
import type { DiffData, VersionInfo } from "~/types/api";

const route = useRoute();
const router = useRouter();
const name = computed(() => route.params.name as string);

const fromVersion = ref((route.query.from as string) || "");
const toVersion = ref((route.query.to as string) || "");

const api = useApi();

// Fetch versions for the pickers
const { data: versionList } = useAsyncData<VersionInfo[]>(
  () => `versions-${name.value}`,
  () => api.fetchVersions(name.value),
  { default: () => [], watch: [name] },
);

const sortedVersions = computed<string[]>(() => versionList.value?.map((v) => v.version) ?? []);

// Build the committed query so the fetch only fires when user submits
const _initialQuery = (() => {
  const f = route.query.from as string | undefined;
  const t = route.query.to as string | undefined;
  return f && t ? { from: f, to: t } : null;
})();
const committedQuery = ref<{ from: string; to: string } | null>(_initialQuery);

function runDiff() {
  const f = fromVersion.value.trim();
  const t = toVersion.value.trim();
  if (!f || !t) return;
  router.replace({ query: { from: f, to: t } });
  committedQuery.value = { from: f, to: t };
}

const canDiff = computed(() => fromVersion.value.trim() && toVersion.value.trim());

const {
  data: diffData,
  status,
  error,
} = useAsyncData<DiffData | null>(
  () => `diff-${name.value}-${committedQuery.value?.from ?? ""}-${committedQuery.value?.to ?? ""}`,
  async () => {
    if (!committedQuery.value) return null;
    return api.fetchDiff(name.value, committedQuery.value.from, committedQuery.value.to);
  },
  { watch: [committedQuery] },
);

const pageTitle = computed(() => {
  if (!committedQuery.value) return `${name.value} — Version Diff — pypx`;
  return `${name.value} ${committedQuery.value.from} → ${committedQuery.value.to} — pypx`;
});

useSeoMeta({
  title: pageTitle,
  description: computed(
    () => `Version diff for ${name.value}: API changes, dependency changes, and changelog.`,
  ),
  ogTitle: pageTitle,
});
</script>

<template>
  <div>
    <div class="mb-6">
      <NuxtLink
        :to="`/packages/${name}`"
        class="text-xs text-muted hover:text-[var(--color-brand)]"
      >
        ← {{ name }}
      </NuxtLink>
      <h1 class="mt-1 text-2xl font-bold tracking-tight text-primary">Version Diff</h1>
      <p class="mt-1 text-sm text-muted">
        Compare API changes, dependency changes, and changelog between two versions.
      </p>
    </div>

    <!-- Version pickers -->
    <div class="mb-6 rounded-[14px] border border-subtle bg-surface p-4">
      <div class="flex flex-wrap items-center gap-3">
        <div class="flex flex-col gap-1">
          <label class="text-[10px] uppercase tracking-[0.1em] text-muted">From</label>
          <select
            v-model="fromVersion"
            class="rounded-lg border border-subtle bg-raised px-3 py-1.5 text-sm text-primary outline-none focus:border-[var(--color-brand-border)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
          >
            <option value="" disabled>select version</option>
            <option v-for="v in sortedVersions" :key="v" :value="v">{{ v }}</option>
          </select>
        </div>

        <span class="mt-5 text-muted">→</span>

        <div class="flex flex-col gap-1">
          <label class="text-[10px] uppercase tracking-[0.1em] text-muted">To</label>
          <select
            v-model="toVersion"
            class="rounded-lg border border-subtle bg-raised px-3 py-1.5 text-sm text-primary outline-none focus:border-[var(--color-brand-border)] focus:ring-1 focus:ring-[var(--color-brand-border)]"
          >
            <option value="" disabled>select version</option>
            <option v-for="v in sortedVersions" :key="v" :value="v">{{ v }}</option>
          </select>
        </div>

        <button
          type="button"
          :disabled="!canDiff"
          class="mt-5 rounded-lg bg-[var(--color-brand)] px-4 py-1.5 text-sm font-semibold text-white transition disabled:opacity-40 hover:opacity-90"
          @click="runDiff"
        >
          Diff
        </button>
      </div>
    </div>

    <!-- Loading skeleton -->
    <div
      v-if="status === 'pending'"
      class="overflow-hidden rounded-[14px] border border-subtle bg-surface"
    >
      <div
        v-for="i in 6"
        :key="i"
        class="h-12 animate-pulse border-b border-subtle bg-raised/30 last:border-b-0"
      />
    </div>

    <!-- Error state -->
    <div
      v-else-if="status === 'error'"
      class="rounded-[14px] border border-subtle bg-surface px-6 py-8 text-center text-sm text-muted"
    >
      Couldn't load diff. Make sure both versions exist and that "from" is older than "to".
    </div>

    <!-- Empty state -->
    <div
      v-else-if="!committedQuery"
      class="rounded-[14px] border border-dashed border-subtle bg-surface px-6 py-12 text-center text-sm text-muted"
    >
      <p>Select two versions above and press Diff.</p>
    </div>

    <!-- Diff sections -->
    <template v-else-if="diffData">
      <DiffSection
        title="Changelog"
        :unavailable="diffData.ChangelogUnavailable"
        :empty="!diffData.Changelog?.length"
        empty-message="No changelog entries in this range."
      >
        <div
          v-for="entry in diffData.Changelog ?? []"
          :key="entry.version || entry.published_at"
          class="border-b border-subtle px-4 py-3 last:border-b-0"
        >
          <div class="flex items-center gap-2">
            <span class="font-mono text-xs font-semibold text-primary">{{
              entry.version || "(unknown)"
            }}</span>
            <span v-if="entry.published_at" class="text-xs text-muted">{{
              entry.published_at
            }}</span>
          </div>
          <p v-if="entry.title" class="mt-0.5 text-sm text-muted">{{ entry.title }}</p>
        </div>
      </DiffSection>

      <DiffSection
        title="Dependency Changes"
        :unavailable="diffData.DepChangesUnavailable"
        :empty="
          !diffData.DepChanges?.Added?.length &&
          !diffData.DepChanges?.Removed?.length &&
          !diffData.DepChanges?.Bumped?.length
        "
        empty-message="No dependency changes."
        class="mt-4"
      >
        <div class="divide-y divide-subtle font-mono text-xs">
          <div
            v-for="dep in diffData.DepChanges?.Added ?? []"
            :key="`added-${dep}`"
            class="flex items-center gap-2 px-4 py-2"
          >
            <span class="text-green-600 dark:text-green-400">+</span>
            <span class="text-primary">{{ dep }}</span>
            <span class="text-muted">added</span>
          </div>
          <div
            v-for="dep in diffData.DepChanges?.Removed ?? []"
            :key="`removed-${dep}`"
            class="flex items-center gap-2 px-4 py-2"
          >
            <span class="text-red-600 dark:text-red-400">−</span>
            <span class="text-primary">{{ dep }}</span>
            <span class="text-muted">removed</span>
          </div>
          <div
            v-for="bump in diffData.DepChanges?.Bumped ?? []"
            :key="`bumped-${bump.Name}`"
            class="flex flex-wrap items-center gap-2 px-4 py-2"
          >
            <span class="text-amber-600 dark:text-amber-400">~</span>
            <span class="text-primary">{{ bump.Name }}</span>
            <span class="text-muted">{{ bump.FromConstraint || "any" }}</span>
            <span class="text-muted">→</span>
            <span class="text-primary">{{ bump.ToConstraint || "any" }}</span>
          </div>
        </div>
      </DiffSection>

      <DiffSection
        title="API Changes"
        :unavailable="diffData.APIChangesUnavailable"
        :empty="
          !diffData.APIChanges?.Added?.length &&
          !diffData.APIChanges?.Removed?.length &&
          !diffData.APIChanges?.Changed?.length
        "
        empty-message="No API changes detected."
        class="mt-4"
      >
        <div class="divide-y divide-subtle font-mono text-xs">
          <div
            v-for="sym in diffData.APIChanges?.Added ?? []"
            :key="`a-${sym}`"
            class="flex items-center gap-2 px-4 py-2"
          >
            <span class="text-green-600 dark:text-green-400">+</span>
            <span class="text-primary">{{ sym }}</span>
          </div>
          <div v-if="(diffData.APIChanges?.AddedTruncated ?? 0) > 0" class="px-4 py-2 text-muted">
            … and {{ diffData.APIChanges!.AddedTruncated }} more added
          </div>
          <div
            v-for="sym in diffData.APIChanges?.Removed ?? []"
            :key="`r-${sym}`"
            class="flex items-center gap-2 px-4 py-2"
          >
            <span class="text-red-600 dark:text-red-400">−</span>
            <span class="text-primary">{{ sym }}</span>
          </div>
          <div v-if="(diffData.APIChanges?.RemovedTruncated ?? 0) > 0" class="px-4 py-2 text-muted">
            … and {{ diffData.APIChanges!.RemovedTruncated }} more removed
          </div>
          <div
            v-for="change in diffData.APIChanges?.Changed ?? []"
            :key="`c-${change.Path}`"
            class="px-4 py-2"
          >
            <div class="flex items-center gap-2">
              <span class="text-amber-600 dark:text-amber-400">~</span>
              <span class="font-semibold text-primary">{{ change.Path }}</span>
            </div>
            <div class="mt-1 ml-4 text-muted">
              <div>
                was: <span class="text-primary">{{ change.FromSig }}</span>
              </div>
              <div>
                now: <span class="text-primary">{{ change.ToSig }}</span>
              </div>
            </div>
          </div>
          <div v-if="(diffData.APIChanges?.ChangedTruncated ?? 0) > 0" class="px-4 py-2 text-muted">
            … and {{ diffData.APIChanges!.ChangedTruncated }} more changed
          </div>
        </div>
      </DiffSection>
    </template>
  </div>
</template>
