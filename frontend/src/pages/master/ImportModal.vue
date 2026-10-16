<script setup lang="ts">
import { ref, watch } from 'vue'
import Modal from '@/components/ui/Modal.vue'
import Icon from '@/components/ui/Icon.vue'
import { ApiError, employees, type ImportResult } from '@/api'
import { useToastStore } from '@/stores/toast'

// Import karyawan dari Excel (H-12): unduh template → isi → unggah. Semua-atau-tidak-sama-sekali.
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; imported: [] }>()
const toast = useToastStore()

const file = ref<File | null>(null)
const busy = ref(false)
const result = ref<ImportResult | null>(null)
const error = ref('')

watch(
  () => props.open,
  (o) => {
    if (o) {
      file.value = null
      result.value = null
      error.value = ''
    }
  },
)

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0] ?? null
  file.value = f
  result.value = null
  error.value = f && !f.name.toLowerCase().endsWith('.xlsx') ? 'Gunakan berkas Excel (.xlsx)' : ''
  if (error.value) file.value = null
}

async function template() {
  try {
    await employees.downloadTemplate()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal mengunduh template')
  }
}

async function run() {
  if (!file.value) return
  busy.value = true
  error.value = ''
  try {
    const r = await employees.importFile(file.value)
    result.value = r
    if (r.errors.length === 0) {
      toast.success(`${r.created} karyawan berhasil diimpor.`)
      emit('imported')
    }
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : 'Gagal mengimpor'
  } finally {
    busy.value = false
  }
}

async function copyCredentials() {
  const r = result.value
  if (!r) return
  const text = [
    'NIK\tEmail\tPassword sementara',
    ...r.credentials.map((c) => `${c.nik}\t${c.email}\t${c.temp_password}`),
  ].join('\n')
  try {
    await navigator.clipboard.writeText(text)
    toast.success('Daftar password disalin.')
  } catch {
    toast.error('Tidak bisa menyalin otomatis; salin manual dari tabel.')
  }
}
</script>

<template>
  <Modal :open="open" title="Import karyawan dari Excel" wide @close="emit('close')">
    <div class="flex flex-col gap-4">
      <ol class="m-0 list-decimal pl-5 text-ink-soft">
        <li>Unduh template, lalu isi satu baris per karyawan.</li>
        <li>Unggah berkas. Jika ada satu baris salah, tidak ada data yang diimpor.</li>
      </ol>
      <div>
        <button type="button" class="btn sm" @click="template"><Icon name="download" />Unduh template</button>
      </div>

      <div v-if="error" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">{{ error }}</div>

      <label
        class="flex cursor-pointer items-center gap-2.5 rounded-xl border-[1.5px] border-dashed border-line-input p-4 text-ink-soft"
      >
        <Icon name="upload" :size="20" />
        <span class="min-w-0 flex-1 truncate">{{ file ? file.name : 'Pilih berkas .xlsx' }}</span>
        <input
          type="file"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          class="sr-only"
          data-testid="import-file"
          @change="onFile"
        />
      </label>

      <div v-if="result && result.errors.length" data-testid="import-errors">
        <p class="m-0 mb-2 font-semibold text-bad-fg">
          {{ result.errors.length }} kesalahan — tidak ada data yang diimpor
        </p>
        <div class="max-h-60 overflow-y-auto rounded-xl border border-line">
          <table class="tbl">
            <thead>
              <tr>
                <th class="w-20">Baris</th>
                <th>Masalah</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(e, i) in result.errors" :key="i">
                <td>{{ e.row || '—' }}</td>
                <td>{{ e.message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="result && !result.errors.length" data-testid="import-ok">
        <p class="m-0 mb-2 font-semibold text-good-fg">
          {{ result.created }} karyawan diimpor<template v-if="result.positions_created"
            >, {{ result.positions_created }} jabatan baru dibuat</template
          >.
        </p>
        <template v-if="result.credentials.length">
          <p class="hint mb-2">
            Password sementara hanya ditampilkan sekali. Bagikan ke karyawan; mereka bisa menggantinya setelah
            masuk.
          </p>
          <div class="max-h-60 overflow-y-auto rounded-xl border border-line">
            <table class="tbl">
              <thead>
                <tr>
                  <th>NIK</th>
                  <th>Email</th>
                  <th>Password sementara</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="c in result.credentials" :key="c.email">
                  <td>{{ c.nik }}</td>
                  <td>{{ c.email }}</td>
                  <td>
                    <code>{{ c.temp_password }}</code>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <button type="button" class="btn sm mt-3" @click="copyCredentials">Salin daftar</button>
        </template>
      </div>
    </div>
    <template #footer>
      <button type="button" class="btn" @click="emit('close')">
        {{ result && !result.errors.length ? 'Selesai' : 'Tutup' }}
      </button>
      <button
        v-if="!(result && !result.errors.length)"
        type="button"
        class="btn pri"
        :disabled="!file || busy"
        data-testid="import-run"
        @click="run"
      >
        {{ busy ? 'Mengimpor…' : 'Impor' }}
      </button>
    </template>
  </Modal>
</template>
