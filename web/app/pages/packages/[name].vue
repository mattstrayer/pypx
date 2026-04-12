<script setup lang="ts">
const route = useRoute();
const name = computed(() => route.params.name as string);
const activeTab = ref("overview");

const api = useApi();
const { data: pkg, status } = await useAsyncData(`package-${name.value}`, () =>
  api.fetchPackage(name.value),
);

// Non-blocking parallel fetches (client-side, don't block SSR)
const { data: security } = useAsyncData(
  `security-${name.value}`,
  () => api.fetchSecurity(name.value, pkg.value?.version),
  { server: false, default: () => null },
);

const { data: extras } = useAsyncData(`extras-${name.value}`, () => api.fetchExtras(name.value), {
  server: false,
  default: () => null,
});

const { data: changelog } = useAsyncData(
  `changelog-${name.value}`,
  () => api.fetchChangelog(name.value).catch(() => null),
  { server: false, default: () => null },
);

// Docs: non-blocking fetch to determine if the Docs tab should be shown.
const { data: docsData } = useAsyncData(
  `docs-${name.value}`,
  () => api.fetchDocs(name.value).catch(() => null),
  { server: false, default: () => null },
);

const repoInfo = computed(() => changelog.value?.repo_info ?? null);

const inPageTabs = [
  { key: "overview", label: "Overview" },
  { key: "dependencies", label: "Dependencies" },
  { key: "versions", label: "Versions" },
  { key: "stats", label: "Stats" },
];

useSeoMeta({
  title: () => (pkg.value ? `${pkg.value.name} — pypx` : "Loading — pypx"),
  description: () => pkg.value?.summary || "",
  ogTitle: () => (pkg.value ? `${pkg.value.name} ${pkg.value.version}` : "pypx"),
  ogDescription: () => pkg.value?.summary || "",
});
</script>

<template>
  <div>
    <!-- Loading state -->
    <div v-if="status === 'pending'" class="flex items-center justify-center py-24">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-zinc-300" />
    </div>

    <!-- Error state -->
    <div v-else-if="status === 'error'" class="py-24 text-center">
      <p class="text-lg font-medium text-zinc-300">Package not found</p>
      <p class="mt-1 text-sm text-zinc-500">No package named "{{ name }}" could be found.</p>
    </div>

    <!-- Loaded state -->
    <div v-else-if="pkg">
      <!-- Header -->
      <div class="mb-6">
        <div class="flex flex-wrap items-baseline gap-3">
          <h1 class="text-3xl font-bold text-zinc-50">{{ pkg.name }}</h1>
          <span class="rounded bg-zinc-800 px-2 py-0.5 font-mono text-sm text-zinc-400">
            v{{ pkg.version }}
          </span>
        </div>
        <p v-if="pkg.summary" class="mt-2 text-zinc-400">{{ pkg.summary }}</p>
        <div class="mt-3">
          <PackageBadges :pkg="pkg" :extras="extras" :security="security" />
        </div>
      </div>

      <!-- Tabs -->
      <div class="mb-6 flex gap-1 overflow-x-auto border-b border-zinc-800 pb-0">
        <!-- In-page tabs -->
        <button
          v-for="tab in inPageTabs"
          :key="tab.key"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium transition-colors"
          :class="
            activeTab === tab.key ? 'bg-zinc-800 text-zinc-50' : 'text-zinc-500 hover:text-zinc-300'
          "
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>

        <!-- Docs tab — link to separate route, shown only when available -->
        <NuxtLink
          v-if="docsData?.available"
          :to="`/packages/${pkg.name}/docs`"
          class="cursor-pointer whitespace-nowrap rounded-t px-4 py-2 text-sm font-medium text-zinc-500 transition-colors hover:text-zinc-300"
        >
          Docs
        </NuxtLink>
      </div>

      <!-- Tab content -->
      <div>
        <div v-if="activeTab === 'overview'">
          <PackageOverview :pkg="pkg" :repo-info="repoInfo" />
        </div>
        <div v-else-if="activeTab === 'dependencies'">
          <PackageDependencies :name="pkg.name" :dependencies="pkg.dependencies" />
        </div>
        <div v-else-if="activeTab === 'versions'"><PackageVersions :name="pkg.name" /></div>
        <div v-else-if="activeTab === 'stats'">
          <PackageStats :name="pkg.name" />
        </div>
      </div>
    </div>
  </div>
</template>
