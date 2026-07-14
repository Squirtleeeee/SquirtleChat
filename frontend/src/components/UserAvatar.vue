<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    src?: string
    name?: string
    size?: number
  }>(),
  { size: 40 },
)

const letter = computed(() => (props.name || '?').charAt(0).toUpperCase())
const style = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  fontSize: `${Math.max(12, props.size * 0.38)}px`,
}))
</script>

<template>
  <div class="user-avatar" :style="style" aria-hidden="true">
    <img v-if="src" :src="src" alt="" class="user-avatar-img" />
    <span v-else>{{ letter }}</span>
  </div>
</template>

<style scoped>
.user-avatar {
  border-radius: 50%;
  background: linear-gradient(135deg, var(--color-primary-soft), #a7f3d0);
  color: var(--color-primary-hover);
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: hidden;
}

.user-avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
</style>
