<script setup lang="ts">
// Pembungkus status muat: memuat / gagal (dengan tombol coba lagi) / kosong / isi.
defineProps<{ loading: boolean; error?: string | null; empty?: boolean; emptyText?: string }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <div v-if="loading" class="card flex items-center gap-3 text-ink-soft" role="status" aria-live="polite">
    <span class="h-5 w-5 animate-spin rounded-full border-2 border-primary border-t-transparent" />
    Memuat…
  </div>
  <div v-else-if="error" class="card border-bad-fg/30 bg-bad-bg/40" role="alert">
    <p class="m-0 font-semibold text-bad-fg">{{ error }}</p>
    <button class="btn sm mt-3" type="button" @click="$emit('retry')">Coba lagi</button>
  </div>
  <div v-else-if="empty" class="card text-center text-ink-soft">{{ emptyText ?? 'Belum ada data.' }}</div>
  <slot v-else />
</template>
