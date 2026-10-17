<script setup lang="ts">
import { ref } from 'vue'
import Modal from '@/components/ui/Modal.vue'
import PageState from '@/components/ui/PageState.vue'
import Icon from '@/components/ui/Icon.vue'
import { ApiError, schedules, type WorkSchedule } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'
import { fmtHours, summarizeSchedule, toMinutes, WEEKDAY_NAMES } from '@/lib/leave'

// H-16 (tab "Jam kerja"): jadwal kerja mingguan. Dipasang per departemen di tab "Departemen & jabatan";
// hari & jam kerja menentukan hitungan hari izin/cuti (Sabtu terhitung bila jadwalnya memuat Sabtu).
const emit = defineEmits<{ changed: [] }>()
const toast = useToastStore()
const { data, loading, error, reload } = useLoader(() => schedules.list())

// tampil Senin → Minggu
const ORDER = [1, 2, 3, 4, 5, 6, 0]

interface Row {
  weekday: number
  on: boolean
  start: string
  end: string
}

const open = ref(false)
const editing = ref<WorkSchedule | null>(null)
const name = ref('')
const isDefault = ref(false)
const breakStart = ref('12:00')
const breakEnd = ref('13:00')
const rows = ref<Row[]>([])
const errors = ref<Record<string, string>>({})
const busy = ref(false)
const removing = ref<WorkSchedule | null>(null)

function openForm(s: WorkSchedule | null) {
  editing.value = s
  name.value = s?.name ?? ''
  isDefault.value = s?.is_default ?? false
  breakStart.value = s ? s.break_start : '12:00'
  breakEnd.value = s ? s.break_end : '13:00'
  rows.value = ORDER.map((wd) => {
    const d = s?.days.find((x) => x.weekday === wd)
    const weekday = wd >= 1 && wd <= 5
    return {
      weekday: wd,
      on: s ? !!d : weekday,
      start: d?.start_time ?? '08:00',
      end: d?.end_time ?? '17:00',
    }
  })
  errors.value = {}
  open.value = true
}

const hoursOf = (r: Row) => {
  const m = toMinutes(r.end) - toMinutes(r.start)
  if (Number.isNaN(m) || m <= 0) return ''
  const a = toMinutes(r.start)
  const b = toMinutes(r.end)
  const bs = breakStart.value ? toMinutes(breakStart.value) : NaN
  const be = breakEnd.value ? toMinutes(breakEnd.value) : NaN
  const brk = Number.isNaN(bs) || Number.isNaN(be) ? 0 : Math.max(0, Math.min(b, be) - Math.max(a, bs))
  return fmtHours(m - brk)
}

async function save() {
  errors.value = {}
  const on = rows.value.filter((r) => r.on)
  const e: Record<string, string> = {}
  if (!name.value.trim()) e.name = 'Nama jadwal wajib diisi'
  if (!on.length) e.days = 'Pilih minimal satu hari kerja'
  if (!!breakStart.value !== !!breakEnd.value)
    e.break_start = 'Isi jam mulai dan selesai istirahat, atau kosongkan keduanya'
  else if (breakStart.value && toMinutes(breakEnd.value) <= toMinutes(breakStart.value))
    e.break_start = 'Jam selesai istirahat harus setelah jam mulai'
  for (const r of on) {
    if (Number.isNaN(toMinutes(r.start)) || Number.isNaN(toMinutes(r.end)))
      e.days = `Isi jam ${WEEKDAY_NAMES[r.weekday]} dengan benar`
    else if (toMinutes(r.end) <= toMinutes(r.start))
      e.days = `Jam selesai ${WEEKDAY_NAMES[r.weekday]} harus setelah jam mulai`
  }
  if (Object.keys(e).length) {
    errors.value = e
    return
  }
  busy.value = true
  try {
    await schedules.save(editing.value?.id ?? null, {
      name: name.value.trim(),
      is_default: isDefault.value,
      break_start: breakStart.value,
      break_end: breakEnd.value,
      days: on.map((r) => ({ weekday: r.weekday, start_time: r.start, end_time: r.end })),
    })
    toast.success('Jadwal kerja disimpan.')
    open.value = false
    await reload()
    emit('changed')
  } catch (err) {
    if (err instanceof ApiError) errors.value = { ...err.fields, form: err.message }
  } finally {
    busy.value = false
  }
}

async function confirmRemove() {
  const s = removing.value
  if (!s) return
  busy.value = true
  try {
    await schedules.remove(s.id)
    toast.success(`${s.name} dihapus.`)
    await reload()
    emit('changed')
  } catch (err) {
    toast.error(err instanceof ApiError ? err.message : 'Gagal menghapus')
  } finally {
    removing.value = null
    busy.value = false
  }
}
</script>

<template>
  <PageState :loading="loading && !data" :error="error" @retry="reload">
    <section v-if="data" class="card !p-2 pb-1" data-testid="schedules">
      <div class="flex items-center gap-3 px-3.5 pb-3 pt-3">
        <div class="flex-1">
          <h2>Jam kerja</h2>
          <p class="hint">
            Pasang jadwal ke departemen di tab “Departemen &amp; jabatan”. Departemen tanpa pilihan memakai
            jadwal default.
          </p>
        </div>
        <button type="button" class="btn sm pri" data-testid="add-schedule" @click="openForm(null)">
          <Icon name="plus" :size="15" />Tambah jadwal
        </button>
      </div>
      <div class="overflow-x-auto">
        <table class="tbl">
          <thead>
            <tr>
              <th>Jadwal</th>
              <th>Hari &amp; jam kerja</th>
              <th><span class="sr-only">Aksi</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in data" :key="s.id">
              <td>
                <b>{{ s.name }}</b>
                <span v-if="s.is_default" class="badge b-teal ml-2 !py-0">default</span>
              </td>
              <td data-testid="summary">
                {{ summarizeSchedule(s.days) }}
                <span v-if="s.break_start" class="muted text-xs" data-testid="break-text"
                  >· istirahat {{ s.break_start }}–{{ s.break_end }}</span
                >
              </td>
              <td class="whitespace-nowrap text-right">
                <button
                  type="button"
                  class="btn sm mr-1.5"
                  :aria-label="`Ubah ${s.name}`"
                  @click="openForm(s)"
                >
                  Ubah
                </button>
                <button
                  type="button"
                  class="btn sm"
                  :aria-label="`Hapus ${s.name}`"
                  :disabled="s.is_default"
                  @click="removing = s"
                >
                  <Icon name="trash" :size="15" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="hint px-3.5 pb-3 pt-1">
        Izin sebagian hari dihitung dari jam efektif: lama izin dibagi jam kerja hari itu, keduanya tanpa jam
        istirahat (mis. 2 jam dari 8 jam efektif = 0,25 hari). Jadwal per shift (mis. untuk Customer Service)
        cukup dibuat di sini lalu dipasang ke departemennya.
      </p>
    </section>
  </PageState>

  <Modal
    :open="open"
    :title="editing ? 'Ubah jadwal kerja' : 'Tambah jadwal kerja'"
    wide
    @close="open = false"
  >
    <form class="flex flex-col gap-4" novalidate @submit.prevent="save">
      <div
        v-if="errors.form && !errors.name && !errors.days && !errors.is_default"
        class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg"
        role="alert"
      >
        {{ errors.form }}
      </div>
      <label class="field"
        >Nama jadwal<input
          v-model="name"
          class="input"
          :class="{ invalid: errors.name }"
          data-testid="s-name"
        /><span v-if="errors.name" class="err">{{ errors.name }}</span></label
      >
      <fieldset class="m-0 flex flex-col gap-2 border-0 p-0">
        <legend class="field mb-1">Hari &amp; jam kerja</legend>
        <div
          v-for="r in rows"
          :key="r.weekday"
          class="grid grid-cols-[110px_1fr_1fr_64px] items-center gap-2 max-sm:grid-cols-[1fr_1fr_1fr]"
        >
          <label class="flex min-h-[44px] items-center gap-2 font-semibold max-sm:col-span-3"
            ><input
              v-model="r.on"
              type="checkbox"
              class="h-[18px] w-[18px]"
              :data-testid="`on-${r.weekday}`"
            />{{ WEEKDAY_NAMES[r.weekday] }}</label
          >
          <input
            v-model="r.start"
            type="time"
            class="input"
            :disabled="!r.on"
            :aria-label="`Jam mulai ${WEEKDAY_NAMES[r.weekday]}`"
            :data-testid="`start-${r.weekday}`"
          />
          <input
            v-model="r.end"
            type="time"
            class="input"
            :disabled="!r.on"
            :aria-label="`Jam selesai ${WEEKDAY_NAMES[r.weekday]}`"
            :data-testid="`end-${r.weekday}`"
          />
          <span class="muted text-xs max-sm:hidden">{{ r.on ? hoursOf(r) : 'libur' }}</span>
        </div>
        <span v-if="errors.days" class="err text-[13px] font-medium text-bad-fg" role="alert">{{
          errors.days
        }}</span>
      </fieldset>
      <div class="grid grid-cols-2 gap-3 sm:gap-4">
        <label class="field"
          >Istirahat mulai <small>tidak dihitung sebagai jam kerja</small>
          <input
            v-model="breakStart"
            type="time"
            class="input"
            :class="{ invalid: errors.break_start }"
            data-testid="break-start"
          />
        </label>
        <label class="field"
          >Istirahat selesai
          <input
            v-model="breakEnd"
            type="time"
            class="input"
            :class="{ invalid: errors.break_start }"
            data-testid="break-end"
          />
        </label>
        <span v-if="errors.break_start" class="col-span-2 text-[13px] font-medium text-bad-fg" role="alert">{{
          errors.break_start
        }}</span>
      </div>
      <label class="flex min-h-[44px] items-center gap-2.5 font-semibold"
        ><input
          v-model="isDefault"
          type="checkbox"
          class="h-[18px] w-[18px]"
          :disabled="editing?.is_default"
        />Jadikan jadwal default perusahaan</label
      >
      <span v-if="errors.is_default" class="err text-[13px] font-medium text-bad-fg">{{
        errors.is_default
      }}</span>
      <button type="submit" class="sr-only" tabindex="-1">Simpan</button>
    </form>
    <template #footer>
      <button type="button" class="btn" @click="open = false">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="s-save" @click="save">
        Simpan
      </button>
    </template>
  </Modal>

  <Modal :open="!!removing" title="Hapus jadwal" @close="removing = null">
    <p class="mt-0">
      Hapus <b>{{ removing?.name }}</b
      >? Jadwal yang masih dipakai departemen tidak dapat dihapus.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="removing = null">Batal</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy"
        data-testid="s-confirm-remove"
        @click="confirmRemove"
      >
        Hapus
      </button>
    </template>
  </Modal>
</template>
