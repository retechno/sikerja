<script setup lang="ts">
import { computed, ref } from 'vue'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import Avatar from '@/components/ui/Avatar.vue'
import { ApiError, auth as authApi } from '@/api'
import { fmtDate, ROLE_LABEL } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'

// Modul 1 — profil: ubah nama/telepon, foto, dan ganti password.
const store = useAuthStore()
const toast = useToastStore()

const roleText = computed(() => (store.user?.roles ?? []).map((r) => ROLE_LABEL[r] ?? r).join(' · '))

// ---- profil ----
const profile = useForm({
  validationSchema: toTypedSchema(
    z.object({
      name: z.string().trim().min(1, 'Nama wajib diisi').max(150, 'Maksimal 150 karakter'),
      phone: z.string().trim().max(30, 'Maksimal 30 karakter').default(''),
    }),
  ),
  initialValues: { name: store.user?.name ?? '', phone: store.user?.phone ?? '' },
})
const [pName] = profile.defineField('name')
const [pPhone] = profile.defineField('phone')
const profileBusy = ref(false)

const saveProfile = profile.handleSubmit(async (v) => {
  profileBusy.value = true
  try {
    store.setUser(await authApi.updateMe(v.name, v.phone ?? ''))
    toast.success('Profil diperbarui.')
  } catch (e) {
    if (e instanceof ApiError) profile.setErrors(e.fields)
    toast.error(e instanceof ApiError ? e.message : 'Gagal menyimpan profil')
  } finally {
    profileBusy.value = false
  }
})

// ---- foto ----
const photoBusy = ref(false)
const photoKey = ref(0)
async function onPhoto(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  input.value = ''
  if (!f) return
  if (!['image/jpeg', 'image/png'].includes(f.type)) return void toast.error('Foto harus JPG atau PNG')
  if (f.size > 2 * 1024 * 1024) return void toast.error('Ukuran foto maksimal 2 MB')
  photoBusy.value = true
  try {
    store.setUser(await authApi.uploadPhoto(f))
    photoKey.value++
    toast.success('Foto diperbarui.')
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : 'Gagal mengunggah foto')
  } finally {
    photoBusy.value = false
  }
}

// ---- password ----
const pw = useForm({
  validationSchema: toTypedSchema(
    z
      .object({
        current: z.string().min(1, 'Password saat ini wajib diisi'),
        next: z.string().min(8, 'Minimal 8 karakter').max(72, 'Maksimal 72 karakter'),
        confirm: z.string().min(1, 'Ulangi password baru'),
      })
      .refine((v) => v.next === v.confirm, { path: ['confirm'], message: 'Password tidak sama' })
      .refine((v) => v.next !== v.current, {
        path: ['next'],
        message: 'Harus berbeda dari password saat ini',
      }),
  ),
})
const [current] = pw.defineField('current')
const [next] = pw.defineField('next')
const [confirm] = pw.defineField('confirm')
const pwBusy = ref(false)
const pwError = ref('')

const savePassword = pw.handleSubmit(async (v) => {
  pwBusy.value = true
  pwError.value = ''
  try {
    await authApi.changePassword(v.current, v.next)
    toast.success('Password diganti. Sesi di perangkat lain diakhiri.')
    pw.resetForm()
  } catch (e) {
    if (e instanceof ApiError) {
      pwError.value = e.message
      const f = e.fields
      pw.setErrors({ current: f.current_password, next: f.new_password })
    } else pwError.value = 'Gagal mengganti password'
  } finally {
    pwBusy.value = false
  }
})
</script>

<template>
  <div v-if="store.user" class="flex flex-wrap items-start gap-6">
    <section class="card min-w-0 flex-[1_1_420px]">
      <div class="mb-5 flex items-center gap-4">
        <Avatar :key="photoKey" :employee="store.user" size="l" />
        <div class="min-w-0">
          <b class="text-base">{{ store.user.name }}</b>
          <div class="muted text-[13px]">{{ roleText }}</div>
          <label class="btn sm mt-2 cursor-pointer" :class="{ 'opacity-50': photoBusy }">
            {{ photoBusy ? 'Mengunggah…' : 'Ganti foto' }}
            <input
              type="file"
              accept="image/jpeg,image/png"
              class="sr-only"
              :disabled="photoBusy"
              data-testid="photo-input"
              @change="onPhoto"
            />
          </label>
        </div>
      </div>

      <form class="flex flex-col gap-4" novalidate @submit.prevent="saveProfile">
        <label class="field"
          >Nama
          <input
            v-model="pName"
            class="input"
            :class="{ invalid: profile.errors.value.name }"
            data-testid="profile-name"
          />
          <span v-if="profile.errors.value.name" class="err">{{ profile.errors.value.name }}</span>
        </label>
        <label class="field"
          >Telepon
          <input v-model="pPhone" class="input" :class="{ invalid: profile.errors.value.phone }" />
          <span v-if="profile.errors.value.phone" class="err">{{ profile.errors.value.phone }}</span>
        </label>
        <dl class="m-0 grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-[13px]">
          <dt class="muted">Email</dt>
          <dd class="m-0">{{ store.user.email }}</dd>
          <dt class="muted">NIK</dt>
          <dd class="m-0">{{ store.user.nik }}</dd>
          <dt class="muted">Departemen</dt>
          <dd class="m-0">{{ store.user.department?.name ?? '—' }}</dd>
          <dt class="muted">Jabatan</dt>
          <dd class="m-0">{{ store.user.position?.name ?? '—' }}</dd>
          <dt class="muted">Tanggal masuk</dt>
          <dd class="m-0">{{ fmtDate(store.user.join_date) }}</dd>
        </dl>
        <p class="hint m-0">Email, NIK, departemen, dan jabatan diatur oleh HR.</p>
        <div class="flex justify-end">
          <button type="submit" class="btn pri" :disabled="profileBusy" data-testid="profile-save">
            Simpan profil
          </button>
        </div>
      </form>
    </section>

    <section class="card min-w-0 flex-[1_1_360px]">
      <h2>Ganti password</h2>
      <p class="hint mb-4">Setelah diganti, sesi di perangkat lain akan berakhir.</p>
      <form class="flex flex-col gap-4" novalidate @submit.prevent="savePassword">
        <div v-if="pwError" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">
          {{ pwError }}
        </div>
        <label class="field"
          >Password saat ini
          <input
            v-model="current"
            type="password"
            autocomplete="current-password"
            class="input"
            :class="{ invalid: pw.errors.value.current }"
            data-testid="pw-current"
          />
          <span v-if="pw.errors.value.current" class="err">{{ pw.errors.value.current }}</span>
        </label>
        <label class="field"
          >Password baru
          <input
            v-model="next"
            type="password"
            autocomplete="new-password"
            class="input"
            :class="{ invalid: pw.errors.value.next }"
            data-testid="pw-new"
          />
          <span v-if="pw.errors.value.next" class="err">{{ pw.errors.value.next }}</span>
        </label>
        <label class="field"
          >Ulangi password baru
          <input
            v-model="confirm"
            type="password"
            autocomplete="new-password"
            class="input"
            :class="{ invalid: pw.errors.value.confirm }"
            data-testid="pw-confirm"
          />
          <span v-if="pw.errors.value.confirm" class="err">{{ pw.errors.value.confirm }}</span>
        </label>
        <div class="flex justify-end">
          <button type="submit" class="btn pri" :disabled="pwBusy" data-testid="pw-save">
            Ganti password
          </button>
        </div>
      </form>
    </section>
  </div>
</template>
