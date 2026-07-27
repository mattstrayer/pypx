<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    keywords?: string[];
    sticky?: boolean;
    placementId?: string;
  }>(),
  { keywords: () => [], sticky: false, placementId: undefined },
);

const { enabled, publisher, type, waitForFill, reload } = useEthicalAds();

const filled = ref(false);

const keywordAttr = computed(() =>
  props.keywords.length > 0 ? props.keywords.join("|") : undefined,
);

const cardClass = computed(() => {
  if (!filled.value) return "";
  const base = "rounded-lg border border-subtle bg-surface p-4";
  return props.sticky ? `sticky top-20 ${base}` : base;
});

onMounted(async () => {
  filled.value = await waitForFill();
});

// Package-to-package navigation keeps this component mounted; without an
// explicit reload the ad would stay pinned to the first package's keywords.
watch(
  () => keywordAttr.value,
  async (next, previous) => {
    if (next === previous) return;
    filled.value = false;
    reload();
    filled.value = await waitForFill();
  },
);
</script>

<template>
  <div v-if="enabled" :class="cardClass">
    <h2 v-if="filled" class="section-label">Sponsored</h2>
    <div
      :id="placementId"
      class="pypx-ad adaptive-css"
      :data-ea-publisher="publisher"
      :data-ea-type="type"
      :data-ea-keywords="keywordAttr"
    />
  </div>
</template>
