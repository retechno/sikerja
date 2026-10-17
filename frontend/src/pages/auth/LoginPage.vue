<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import { ApiError } from '@/api'
import { useAuthStore } from '@/stores/auth'

// H-01 — Login. Token disimpan server sebagai cookie httpOnly; JS tidak pernah memegangnya.
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()

const schema = toTypedSchema(
  z.object({
    email: z.string().min(1, 'Email wajib diisi').email('Format email tidak valid'),
    password: z.string().min(1, 'Password wajib diisi'),
    remember: z.boolean().default(true),
  }),
)
const { defineField, handleSubmit, errors } = useForm({
  validationSchema: schema,
  initialValues: { remember: true },
})
const [email, emailAttrs] = defineField('email')
const [password, passwordAttrs] = defineField('password')
const [remember] = defineField('remember')

const busy = ref(false)
const serverError = ref('')
const lockLeft = ref(0) // detik tersisa saat terkunci (HTTP 429)
let timer: ReturnType<typeof setInterval> | undefined

function startCountdown(seconds: number) {
  lockLeft.value = seconds
  clearInterval(timer)
  timer = setInterval(() => {
    lockLeft.value -= 1
    if (lockLeft.value <= 0) {
      clearInterval(timer)
      serverError.value = ''
    }
  }, 1000)
}
onBeforeUnmount(() => clearInterval(timer))

const locked = computed(() => lockLeft.value > 0)

const submit = handleSubmit(async (v) => {
  busy.value = true
  serverError.value = ''
  try {
    await auth.login(v.email, v.password, v.remember)
    const target =
      typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/')
        ? route.query.redirect
        : auth.home
    await router.replace(target)
  } catch (e) {
    if (e instanceof ApiError && e.status === 429) {
      serverError.value = 'Terlalu banyak percobaan masuk.'
      startCountdown(e.retryAfter || 60)
    } else {
      serverError.value = e instanceof ApiError ? e.message : 'Gagal masuk. Coba lagi.'
    }
  } finally {
    busy.value = false
  }
})
</script>

<template>
  <div class="flex min-h-screen flex-wrap">
    <div
      class="flex flex-[1_1_520px] flex-col justify-between gap-12 bg-side px-8 py-10 text-[#DCE6E3] sm:px-16 sm:py-14"
    >
      <div class="flex items-center gap-3 text-[22px] font-extrabold text-white">
        <span class="flex h-10 w-10 items-center justify-center rounded-[11px] bg-accent text-xl text-side"
          >S</span
        >
        SiKerja
      </div>
      <div class="flex max-w-[520px] flex-col gap-5">
        <h1 class="m-0 text-[34px] font-extrabold leading-[1.12] tracking-tight text-white sm:text-[44px]">
          Kinerja terukur, cuti tanpa kertas.
        </h1>
        <p class="m-0 text-[17px] text-[#B7C7C2]">
          Satu tempat untuk KPI, penilaian, dan pengajuan izin seluruh tim IT Support, Customer Service, Sales
          &amp; Marketing, dan Developer.
        </p>
        <div class="mt-3 grid grid-cols-3 gap-3 text-center sm:text-left">
          <div class="rounded-[14px] bg-side-hover p-4">
            <div class="text-lg font-bold text-white">KPI</div>
            <div class="text-[13px] text-[#B7C7C2]">per departemen</div>
          </div>
          <div class="rounded-[14px] bg-side-hover p-4">
            <div class="text-lg font-bold text-white">360</div>
            <div class="text-[13px] text-[#B7C7C2]">nilai perilaku</div>
          </div>
          <div class="rounded-[14px] bg-side-hover p-4">
            <div class="text-lg font-bold text-accent">Bulanan</div>
            <div class="text-[13px] text-[#B7C7C2]">siklus penilaian</div>
          </div>
        </div>
      </div>
      <div class="text-[13px] text-[#7F9792]">Aplikasi internal · akses hanya untuk karyawan</div>
    </div>

    <div class="flex flex-[1_1_480px] items-center justify-center px-6 py-14">
      <form class="flex w-full max-w-[400px] flex-col gap-5" novalidate @submit.prevent="submit">
        <div>
          <h2 class="m-0 text-[28px] font-bold tracking-tight">Masuk</h2>
          <p class="mb-0 mt-1.5 text-ink-soft">Gunakan email kantor Anda.</p>
        </div>

        <div
          v-if="serverError"
          class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg"
          role="alert"
          data-testid="login-error"
        >
          {{ serverError }}
          <template v-if="locked">
            Coba lagi dalam <b>{{ lockLeft }}</b> detik.</template
          >
        </div>

        <label class="field">
          Email
          <input
            v-model="email"
            v-bind="emailAttrs"
            type="email"
            autocomplete="username"
            class="input !min-h-[48px] !rounded-xl"
            :class="{ invalid: errors.email }"
            placeholder="nama@perusahaan.co.id"
            :aria-invalid="!!errors.email"
          />
          <span v-if="errors.email" class="err">{{ errors.email }}</span>
        </label>

        <label class="field">
          Password
          <input
            v-model="password"
            v-bind="passwordAttrs"
            type="password"
            autocomplete="current-password"
            class="input !min-h-[48px] !rounded-xl"
            :class="{ invalid: errors.password }"
            :aria-invalid="!!errors.password"
          />
          <span v-if="errors.password" class="err">{{ errors.password }}</span>
        </label>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <label class="flex min-h-[44px] items-center gap-2 text-[13px]">
            <input v-model="remember" type="checkbox" class="h-[18px] w-[18px]" />Ingat saya
          </label>
          <RouterLink to="/forgot-password" class="text-[13px] font-semibold">Lupa password?</RouterLink>
        </div>

        <button
          type="submit"
          class="btn pri !min-h-[48px] !text-[15px] !font-bold"
          :disabled="busy || locked"
        >
          {{ busy ? 'Memeriksa…' : locked ? `Tunggu ${lockLeft} detik` : 'Masuk' }}
        </button>
        <p class="m-0 text-center text-[12.5px] text-ink-soft">
          Belum punya akun? Hubungi HR untuk dibuatkan.
        </p>
      </form>
    </div>
  </div>
</template>
