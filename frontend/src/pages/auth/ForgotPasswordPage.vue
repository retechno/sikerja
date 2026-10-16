<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import { ApiError, auth } from '@/api'

const { defineField, handleSubmit, errors } = useForm({
  validationSchema: toTypedSchema(
    z.object({ email: z.string().min(1, 'Email wajib diisi').email('Format email tidak valid') }),
  ),
})
const [email, attrs] = defineField('email')
const busy = ref(false)
const done = ref(false)
const error = ref('')

const submit = handleSubmit(async (v) => {
  busy.value = true
  error.value = ''
  try {
    await auth.forgot(v.email)
    done.value = true
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Gagal mengirim permintaan'
  } finally {
    busy.value = false
  }
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center px-4 py-10">
    <div class="card w-full max-w-[420px] !p-7">
      <h1 class="m-0 text-2xl font-bold tracking-tight">Lupa password</h1>
      <template v-if="!done">
        <p class="mb-5 mt-2 text-ink-soft">
          Masukkan email kantor Anda. Kami kirim tautan untuk mengatur ulang password (berlaku 1 jam).
        </p>
        <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
          <div v-if="error" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">{{ error }}</div>
          <label class="field">
            Email
            <input
              v-model="email"
              v-bind="attrs"
              type="email"
              class="input"
              :class="{ invalid: errors.email }"
              autocomplete="username"
            />
            <span v-if="errors.email" class="err">{{ errors.email }}</span>
          </label>
          <button type="submit" class="btn pri" :disabled="busy">
            {{ busy ? 'Mengirim…' : 'Kirim tautan' }}
          </button>
        </form>
      </template>
      <div
        v-else
        class="mt-4 rounded-xl bg-good-bg px-4 py-3.5 text-good-fg"
        role="status"
        data-testid="forgot-done"
      >
        Jika email terdaftar, tautan pengaturan ulang password sudah dikirim. Periksa kotak masuk Anda.
      </div>
      <p class="mb-0 mt-5 text-center">
        <RouterLink to="/login" class="font-semibold">← Kembali ke halaman masuk</RouterLink>
      </p>
    </div>
  </div>
</template>
