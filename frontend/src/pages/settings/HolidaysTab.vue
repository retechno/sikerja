<script setup lang="ts">
import { ref } from 'vue'
import Modal from '@/components/ui/Modal.vue'
import PageState from '@/components/ui/PageState.vue'
import Icon from '@/components/ui/Icon.vue'
import HolidayImportModal from './HolidayImportModal.vue'
import { ApiError, leave, type Holiday } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'
import { toDateInput } from '@/lib/format'
import { weekdayOf } from '@/lib/leave'

// H-16 (tab "Hari libur"): libur nasional & cuti bersama; tidak dihitung sebagai hari kerja.
const toast = useToastStore()
const thisYear = new Date().getFullYear()
const year = ref(thisYear)
const years = [thisYear - 1, thisYear, thisYear + 1]
const { data, loading, error, reload } = useLoader(() => leave.holidays(year.value), [year])

const DAY = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu']
const MON = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des']
const dayNum = (iso: string) => Number(iso.slice(8, 10))
const mon = (iso: string) => MON[Number(iso.slice(5, 7)) - 1]

const open = ref(false)
const editing = ref<Holiday | null>(null)
const form = ref({ date: '', name: '', type: 'nasional' as Holiday['type'] })
const errors = ref<Record<string, string>>({})
const busy = ref(false)
const removing = ref<Holiday | null>(null)
const importing = ref(false)

function openForm(h: Holiday | null) {
  editing.value = h
  form.value = h
    ? { date: toDateInput(h.date), name: h.name, type: h.type }
    : { date: `${year.value}-01-01`, name: '', type: 'nasional' }
  errors.value = {}
  open.value = true
}

async function save() {
  errors.value = {}
  const e: Record<string, string> = {}
  if (!form.value.date) e.date = 'Tanggal wajib diisi'
  if (!form.value.name.trim()) e.name = 'Nama wajib diisi'
  if (Object.keys(e).length) {
    errors.value = e
    return
  }
  busy.value = true
  try {
    await leave.saveHoliday(editing.value?.id ?? null, { ...form.value, name: form.value.name.trim() })
    toast.success('Hari libur disimpan.')
    open.value = false
    await reload()
  } catch (err) {
    if (err instanceof ApiError) errors.value = { ...err.fields, form: err.message }
  } finally {
    busy.value = false
  }
}

async function confirmRemove() {
  const h = removing.value
  if (!h) return
  busy.value = true
  try {
    await leave.deleteHoliday(h.id)
    toast.success('Hari libur dihapus.')
    await reload()
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : 'Gagal menghapus')
  } finally {
    removing.value = null
    busy.value = false
  }
}

async function importOfficial() {
  importing.value = false
  busy.value = true
  try {
    const r = await leave.importHolidays(year.value)
    toast.success(
      r.added
        ? `${r.added} hari libur ditambahkan. Mohon cek ulang dengan SKB terbaru.`
        : 'Semua tanggal sudah ada.',
    )
    await reload()
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : 'Gagal mengimpor')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="card" data-testid="holidays">
    <div class="mb-3 flex flex-wrap items-center gap-3">
      <div class="min-w-0 flex-1">
        <h2>Hari libur</h2>
        <p class="hint">Tidak dihitung sebagai hari kerja pada pengajuan izin &amp; cuti</p>
      </div>
      <label class="flex items-center gap-2 text-[13px] font-semibold"
        >Tahun
        <select
          v-model.number="year"
          class="input !min-h-[40px] !w-auto"
          aria-label="Tahun"
          data-testid="h-year"
        >
          <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
        </select>
      </label>
      <button type="button" class="btn sm" :disabled="busy" data-testid="import" @click="importing = true">
        Impor hari libur
      </button>
      <button type="button" class="btn sm pri" data-testid="add-holiday" @click="openForm(null)">
        <Icon name="plus" :size="15" />Tambah
      </button>
    </div>

    <PageState
      :loading="loading && !data"
      :error="error"
      :empty="!!data && !data.length"
      :empty-text="`Belum ada hari libur untuk ${year}. Impor daftar resmi atau tambahkan setelah SKB pemerintah terbit.`"
      @retry="reload"
    >
      <div
        v-for="h in data"
        :key="h.id"
        class="flex items-center gap-3.5 border-b border-line-soft py-2.5 last:border-b-0"
        :data-testid="`holiday-${h.id}`"
      >
        <div class="w-12 flex-none rounded-xl bg-primary-soft py-1 text-center leading-tight">
          <b class="block text-base text-primary">{{ dayNum(h.date) }}</b>
          <span class="text-[11px] text-ink-soft">{{ mon(h.date) }}</span>
        </div>
        <div class="min-w-0 flex-1">
          <b class="block truncate">{{ h.name }}</b>
          <span class="muted text-[12.5px]">{{ DAY[weekdayOf(h.date)] }}</span>
        </div>
        <span class="badge" :class="h.type === 'nasional' ? 'b-info' : 'b-warn'">{{
          h.type === 'nasional' ? 'Nasional' : 'Cuti bersama'
        }}</span>
        <button type="button" class="btn sm" :aria-label="`Ubah ${h.name}`" @click="openForm(h)">Ubah</button>
        <button type="button" class="btn sm" :aria-label="`Hapus ${h.name}`" @click="removing = h">
          <Icon name="trash" :size="15" />
        </button>
      </div>
    </PageState>
    <p class="hint mb-0 mt-3">
      Daftar hasil impor bisa berubah mengikuti SKB Tiga Menteri; HR wajib mengecek ulang sebelum dipakai.
    </p>
  </section>

  <HolidayImportModal
    :open="importing"
    :year="year"
    @close="importing = false"
    @done="((importing = false), reload())"
    @builtin="importOfficial"
  />

  <Modal :open="open" :title="editing ? 'Ubah hari libur' : 'Tambah hari libur'" @close="open = false">
    <form class="flex flex-col gap-4" novalidate @submit.prevent="save">
      <div
        v-if="errors.form && !errors.date && !errors.name"
        class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg"
        role="alert"
      >
        {{ errors.form }}
      </div>
      <label class="field"
        >Tanggal<input
          v-model="form.date"
          type="date"
          class="input"
          :class="{ invalid: errors.date }"
          data-testid="h-date"
        /><span v-if="errors.date" class="err">{{ errors.date }}</span></label
      >
      <label class="field"
        >Nama<input
          v-model="form.name"
          class="input"
          :class="{ invalid: errors.name }"
          data-testid="h-name"
        /><span v-if="errors.name" class="err">{{ errors.name }}</span></label
      >
      <label class="field"
        >Jenis
        <select v-model="form.type" class="input">
          <option value="nasional">Libur nasional</option>
          <option value="cuti_bersama">Cuti bersama</option>
        </select>
      </label>
      <button type="submit" class="sr-only" tabindex="-1">Simpan</button>
    </form>
    <template #footer>
      <button type="button" class="btn" @click="open = false">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="h-save" @click="save">
        Simpan
      </button>
    </template>
  </Modal>

  <Modal :open="!!removing" title="Hapus hari libur" @close="removing = null">
    <p class="mt-0">
      Hapus <b>{{ removing?.name }}</b
      >? Pengajuan yang sudah ada tidak dihitung ulang.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="removing = null">Batal</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy"
        data-testid="h-confirm-remove"
        @click="confirmRemove"
      >
        Hapus
      </button>
    </template>
  </Modal>
</template>
