<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
</script>

<template>
  <div class="card text-center">
    <h2 class="text-xl">Halaman tidak ditemukan</h2>
    <p class="muted">Alamat yang Anda buka tidak ada atau sudah dipindahkan.</p>
    <RouterLink :to="auth.home" class="btn pri mt-2">Kembali ke beranda</RouterLink>
  </div>
</template>
