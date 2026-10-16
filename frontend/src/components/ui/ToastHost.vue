<script setup lang="ts">
import { useToastStore } from '@/stores/toast'

const toast = useToastStore()
const cls = {
  success: 'bg-good-bg text-good-fg',
  error: 'bg-bad-bg text-bad-fg',
  info: 'bg-info-bg text-info-fg',
}
</script>

<template>
  <div
    class="pointer-events-none fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4"
    aria-live="polite"
  >
    <div
      v-for="t in toast.items"
      :key="t.id"
      class="pointer-events-auto flex max-w-md items-start gap-3 rounded-xl px-4 py-3 text-sm font-medium shadow-lg"
      :class="cls[t.kind]"
      role="status"
    >
      <span class="flex-1">{{ t.text }}</span>
      <button
        type="button"
        class="font-bold opacity-70 hover:opacity-100"
        aria-label="Tutup"
        @click="toast.dismiss(t.id)"
      >
        ×
      </button>
    </div>
  </div>
</template>
