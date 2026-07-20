<template>
  <NuxtLayout>
    <NuxtPage :page-key="(route) => route.path" />
  </NuxtLayout>
  <CommandPalette />
</template>
