<script setup lang="ts">
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import { ApiError, auth } from '@/api'
import { useToastStore } from '@/stores/toast'

const route = useRoute()
const router = useRouter()
const toast = useToastStore()
const token = typeof route.query.token === 'string' ? route.query.token : ''

const schema = toTypedSchema(
  z
    .object({
      password: z.string().min(8, 'Minimal 8 karakter').max(72, 'Maksimal 72 karakter'),
      confirm: z.string().min(1, 'Ulangi password baru'),
    })
    .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'Password tidak sama' }),
)
const { defineField, handleSubmit, errors } = useForm({ validationSchema: schema })
const [password, pAttrs] = defineField('password')
const [confirm, cAttrs] = defineField('confirm')
const busy = ref(false)
const error = ref('')

const submit = handleSubmit(async (v) => {
  busy.value = true
  error.value = ''
  try {
    await auth.reset(token, v.password)
    toast.success('Password berhasil diubah. Silakan masuk.')
    await router.replace({ name: 'login' })
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Gagal mengatur ulang password'
  } finally {
    busy.value = false
  }
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center px-4 py-10">
    <div class="card w-full max-w-[420px] !p-7">
      <h1 class="m-0 text-2xl font-bold tracking-tight">Atur ulang password</h1>
      <div v-if="!token" class="mt-4 rounded-xl bg-bad-bg px-4 py-3.5 text-bad-fg" role="alert">
        Tautan tidak lengkap. Minta tautan baru dari halaman
        <RouterLink to="/forgot-password" class="font-semibold">Lupa password</RouterLink>.
      </div>
      <form v-else class="mt-5 flex flex-col gap-4" novalidate @submit.prevent="submit">
        <div v-if="error" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">{{ error }}</div>
        <label class="field">
          Password baru
          <input
            v-model="password"
            v-bind="pAttrs"
            type="password"
            class="input"
            :class="{ invalid: errors.password }"
            autocomplete="new-password"
          />
          <span v-if="errors.password" class="err">{{ errors.password }}</span>
        </label>
        <label class="field">
          Ulangi password baru
          <input
            v-model="confirm"
            v-bind="cAttrs"
            type="password"
            class="input"
            :class="{ invalid: errors.confirm }"
            autocomplete="new-password"
          />
          <span v-if="errors.confirm" class="err">{{ errors.confirm }}</span>
        </label>
        <button type="submit" class="btn pri" :disabled="busy">
          {{ busy ? 'Menyimpan…' : 'Simpan password' }}
        </button>
      </form>
      <p class="mb-0 mt-5 text-center">
        <RouterLink to="/login" class="font-semibold">← Kembali ke halaman masuk</RouterLink>
      </p>
    </div>
  </div>
</template>
