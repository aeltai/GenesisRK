<script setup lang="ts">
withDefaults(
  defineProps<{
    size?: 'sm' | 'md' | 'lg'
    label?: string
  }>(),
  { size: 'md', label: '' }
)
</script>

<template>
  <div class="loading-shapes" :class="size" role="status" :aria-label="label || 'Loading'">
    <div class="shapes" aria-hidden="true"></div>
    <p v-if="label" class="loading-shapes-label">{{ label }}</p>
  </div>
</template>

<style scoped>
.loading-shapes {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
}

.loading-shapes.sm {
  --loader-size: 28px;
}

.loading-shapes.md {
  --loader-size: 56px;
}

.loading-shapes.lg {
  --loader-size: 68px;
}

.shapes {
  width: var(--loader-size);
  height: var(--loader-size);
  color: var(--accent);
  background:
    linear-gradient(currentColor 0 0) left calc(var(--loader-size) / 3) top 0,
    linear-gradient(currentColor 0 0) top calc(var(--loader-size) / 3) right 0,
    linear-gradient(currentColor 0 0) right calc(var(--loader-size) / 3) bottom 0,
    linear-gradient(currentColor 0 0) bottom calc(var(--loader-size) / 3) left 0;
  background-size: calc(100% / 3) calc(100% / 3);
  background-repeat: no-repeat;
  animation:
    shapes-morph 0.75s infinite alternate linear,
    shapes-flip 1.5s infinite;
}

@keyframes shapes-morph {
  90%,
  100% {
    background-size:
      calc(2 * 100% / 3) calc(100% / 3),
      calc(100% / 3) calc(2 * 100% / 3);
  }
}

@keyframes shapes-flip {
  0%,
  49.99% {
    transform: scaleX(1);
  }

  50%,
  100% {
    transform: scaleX(-1);
  }
}

.loading-shapes-label {
  margin: 0;
  font-size: 0.9rem;
  color: var(--text-muted);
}
</style>
