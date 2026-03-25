<script setup lang="ts">
defineProps<{
  categories?: string[];
}>();
</script>

<template>
  <div class="category-switcher">
    <slot />
  </div>
</template>
