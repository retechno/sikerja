<script setup lang="ts">
import { computed, ref } from 'vue'
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import { z } from 'zod'
import PageState from '@/components/ui/PageState.vue'
import Modal from '@/components/ui/Modal.vue'
import { ApiError, periods, type Period } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { fmtDate, fmtNum, fmtScore, PERIOD_STATUS, toDateInput } from '@/lib/format'
import { usePeriodStore } from '@/stores/period'
import { useToastStore } from '@/stores/toast'

// H-14 — Periode Penilaian (HR): buat, buka, tutup, buka kembali (diaudit), atur bobot KPI vs perilaku.
const toast = useToastStore()
const periodStore = usePeriodStore()

const { data, loading, error, reload } = useLoader(async () => {
  const [list, progress] = await Promise.all([periods.list(), periods.progress()])
  return {
    list: [...list].sort((a, b) => b.start_date.localeCompare(a.start_date)),
    progress: new Map(progress.map((p) => [p.period_id, p])),
  }
})

const activePeriod = computed(() => data.value?.list.find((p) => p.status === 'active') ?? null)
const activeProgress = computed(() =>
  activePeriod.value ? data.value?.progress.get(activePeriod.value.id) : undefined,
)
const pct = (n: number, d: number) => (d > 0 ? Math.min(100, Math.round((n / d) * 100)) : 0)

// --- form ---
const schema = toTypedSchema(
  z
    .object({
      name: z.string().trim().min(1, 'Nama periode wajib diisi'),
      type: z.enum(['monthly', 'quarterly']),
      start_date: z.string().min(1, 'Tanggal mulai wajib diisi'),
      end_date: z.string().min(1, 'Tanggal selesai wajib diisi'),
      kpi_weight: z.number({ invalid_type_error: 'Isi angka' }).min(0, 'Min. 0').max(100, 'Maks. 100'),
      behavior_weight: z.number({ invalid_type_error: 'Isi angka' }).min(0, 'Min. 0').max(100, 'Maks. 100'),
    })
    .refine((v) => v.end_date >= v.start_date, {
      path: ['end_date'],
      message: 'Tanggal selesai tidak boleh sebelum tanggal mulai',
    })
    .refine((v) => Math.abs(v.kpi_weight + v.behavior_weight - 100) < 0.001, {
      path: ['behavior_weight'],
      message: 'Bobot KPI + perilaku harus 100%',
    }),
)
const { defineField, handleSubmit, errors, resetForm, setErrors } = useForm({
  validationSchema: schema,
  initialValues: { type: 'monthly' as const, kpi_weight: 70, behavior_weight: 30 },
})
const [name] = defineField('name')
const [type] = defineField('type')
const [startDate] = defineField('start_date')
const [endDate] = defineField('end_date')
const [kpiWeight] = defineField('kpi_weight')
const [behaviorWeight] = defineField('behavior_weight')

const editingId = ref<number | null>(null)
const busy = ref(false)
const formError = ref('')

function newForm() {
  editingId.value = null
  formError.value = ''
  resetForm({
    values: { name: '', type: 'monthly', start_date: '', end_date: '', kpi_weight: 70, behavior_weight: 30 },
  })
}
function editPeriod(p: Period) {
  editingId.value = p.id
  formError.value = ''
  resetForm({
    values: {
      name: p.name,
      type: p.type,
      start_date: toDateInput(p.start_date),
      end_date: toDateInput(p.end_date),
      kpi_weight: Number(p.kpi_weight),
      behavior_weight: Number(p.behavior_weight),
    },
  })
  document.getElementById('period-form')?.scrollIntoView({ behavior: 'smooth' })
}
// bobot saling melengkapi: mengubah satu mengisi yang lain
function onKpi(e: Event) {
  const v = Number((e.target as HTMLInputElement).value)
  kpiWeight.value = v
  if (Number.isFinite(v) && v >= 0 && v <= 100) behaviorWeight.value = Math.round((100 - v) * 100) / 100
}
function onBehavior(e: Event) {
  const v = Number((e.target as HTMLInputElement).value)
  behaviorWeight.value = v
  if (Number.isFinite(v) && v >= 0 && v <= 100) kpiWeight.value = Math.round((100 - v) * 100) / 100
}

const submit = handleSubmit(async (v) => {
  busy.value = true
  formError.value = ''
  try {
    if (editingId.value) await periods.update(editingId.value, v)
    else await periods.create(v)
    toast.success(editingId.value ? 'Periode diperbarui.' : 'Periode disimpan sebagai draf.')
    newForm()
    await Promise.all([reload(), periodStore.load(true)])
  } catch (e) {
    if (e instanceof ApiError) {
      formError.value = e.message
      if (Object.keys(e.fields).length) setErrors(e.fields)
    } else formError.value = 'Gagal menyimpan'
  } finally {
    busy.value = false
  }
})

// --- aksi status ---
type Act = 'open' | 'close' | 'reopen'
const confirm = ref<{ period: Period; act: Act } | null>(null)
const ACT_TEXT: Record<Act, { title: string; body: string; btn: string }> = {
  open: {
    title: 'Buka periode',
    body: 'Periode menjadi Aktif: karyawan dapat mulai menginput realisasi dan melakukan penilaian.',
    btn: 'Buka periode',
  },
  close: {
    title: 'Tutup periode',
    body: 'Data periode terkunci: realisasi dan penilaian tidak dapat diubah lagi. Hanya HR yang dapat membukanya kembali (tercatat di audit log).',
    btn: 'Tutup periode',
  },
  reopen: {
    title: 'Buka kembali periode',
    body: 'Periode kembali Aktif dan data dapat diubah lagi. Tindakan ini tercatat di audit log.',
    btn: 'Buka kembali',
  },
}

async function runAct() {
  if (!confirm.value) return
  const { period, act } = confirm.value
  busy.value = true
  try {
    await periods[act](period.id)
    toast.success(
      `${period.name}: ${act === 'open' ? 'dibuka' : act === 'close' ? 'ditutup' : 'dibuka kembali'}.`,
    )
    confirm.value = null
    await Promise.all([reload(), periodStore.load(true)])
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal mengubah status')
    confirm.value = null
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PageState :loading="loading && !data" :error="error" @retry="reload">
    <template v-if="data">
      <section v-if="activePeriod" class="card" data-testid="active-card">
        <div class="mb-4 flex flex-wrap items-center gap-3">
          <span class="badge b-good">Aktif</span>
          <div class="min-w-0 flex-1">
            <b class="text-base">{{ activePeriod.name }}</b>
            <div class="muted text-[13px]">
              {{ fmtDate(activePeriod.start_date) }} – {{ fmtDate(activePeriod.end_date) }} ·
              {{ activePeriod.type === 'monthly' ? 'bulanan' : 'kuartalan' }} · KPI
              {{ fmtNum(activePeriod.kpi_weight) }}% + perilaku {{ fmtNum(activePeriod.behavior_weight) }}%
            </div>
          </div>
          <button type="button" class="btn sm" @click="editPeriod(activePeriod)">Ubah bobot</button>
          <button
            type="button"
            class="btn sm danger"
            data-testid="close-active"
            @click="confirm = { period: activePeriod, act: 'close' }"
          >
            Tutup periode
          </button>
        </div>
        <div v-if="activeProgress" class="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
          <div
            v-for="m in [
              {
                k: 'Realisasi KPI terisi',
                n: activeProgress.results,
                d: activeProgress.expected_results,
                u: 'indikator',
              },
              {
                k: 'Tervalidasi atasan',
                n: activeProgress.validated,
                d: activeProgress.expected_results,
                u: 'indikator',
              },
              {
                k: 'Self-assessment',
                n: activeProgress.self_done,
                d: activeProgress.employees,
                u: 'karyawan',
              },
              {
                k: 'Penilaian atasan',
                n: activeProgress.manager_done,
                d: activeProgress.employees,
                u: 'karyawan',
              },
            ]"
            :key="m.k"
          >
            <div class="k">{{ m.k }}</div>
            <div class="text-2xl font-bold">{{ pct(m.n, m.d) }}%</div>
            <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-line-soft">
              <div class="h-full rounded-full bg-primary" :style="{ width: pct(m.n, m.d) + '%' }" />
            </div>
            <div class="muted mt-1 text-xs">{{ m.n }} dari {{ m.d }} {{ m.u }}</div>
          </div>
        </div>
      </section>

      <div class="flex flex-wrap items-start gap-6">
        <section class="card min-w-0 flex-[2_1_560px] !p-2 pb-1">
          <div class="px-3.5 pb-3 pt-3"><h2>Semua periode</h2></div>
          <div class="overflow-x-auto">
            <table class="tbl" data-testid="period-table">
              <thead>
                <tr>
                  <th>Periode</th>
                  <th>Rentang</th>
                  <th class="num">Bobot KPI / perilaku</th>
                  <th class="num">Rata-rata</th>
                  <th>Status</th>
                  <th><span class="sr-only">Aksi</span></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="p in data.list" :key="p.id" :data-testid="`period-${p.id}`">
                  <td>
                    <b>{{ p.name }}</b>
                  </td>
                  <td class="whitespace-nowrap">{{ fmtDate(p.start_date) }} – {{ fmtDate(p.end_date) }}</td>
                  <td class="num">{{ fmtNum(p.kpi_weight) }} / {{ fmtNum(p.behavior_weight) }}</td>
                  <td class="num">
                    {{
                      data.progress.get(p.id)?.avg_final
                        ? fmtScore(data.progress.get(p.id)?.avg_final)
                        : p.status === 'active'
                          ? 'berjalan'
                          : '—'
                    }}
                  </td>
                  <td>
                    <span class="badge" :class="PERIOD_STATUS[p.status]?.cls">{{
                      PERIOD_STATUS[p.status]?.label
                    }}</span>
                  </td>
                  <td class="whitespace-nowrap text-right">
                    <button
                      v-if="p.status !== 'closed'"
                      type="button"
                      class="btn sm mr-1.5"
                      @click="editPeriod(p)"
                    >
                      Ubah
                    </button>
                    <button
                      v-if="p.status === 'draft'"
                      type="button"
                      class="btn sm pri"
                      data-act="open"
                      @click="confirm = { period: p, act: 'open' }"
                    >
                      Buka
                    </button>
                    <button
                      v-else-if="p.status === 'active'"
                      type="button"
                      class="btn sm"
                      data-act="close"
                      @click="confirm = { period: p, act: 'close' }"
                    >
                      Tutup
                    </button>
                    <button
                      v-else
                      type="button"
                      class="btn sm"
                      data-act="reopen"
                      @click="confirm = { period: p, act: 'reopen' }"
                    >
                      Buka kembali
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p class="hint px-3.5 pb-3 pt-2">Membuka kembali periode yang ditutup tercatat di audit log.</p>
        </section>

        <section id="period-form" class="card min-w-0 flex-[1_1_340px]">
          <h2>{{ editingId ? 'Ubah periode' : 'Buat periode baru' }}</h2>
          <p class="hint mb-4">Disimpan sebagai Draf sampai diaktifkan</p>
          <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
            <div v-if="formError" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">
              {{ formError }}
            </div>
            <label class="field"
              >Nama periode
              <input
                v-model="name"
                class="input"
                :class="{ invalid: errors.name }"
                placeholder="mis. November 2026"
                data-testid="p-name"
              />
              <span v-if="errors.name" class="err">{{ errors.name }}</span>
            </label>
            <label class="field"
              >Tipe
              <select v-model="type" class="input">
                <option value="monthly">Bulanan</option>
                <option value="quarterly">Kuartalan</option>
              </select>
            </label>
            <div class="grid grid-cols-2 gap-3">
              <label class="field"
                >Mulai
                <input
                  v-model="startDate"
                  type="date"
                  class="input"
                  :class="{ invalid: errors.start_date }"
                  data-testid="p-start"
                />
                <span v-if="errors.start_date" class="err">{{ errors.start_date }}</span>
              </label>
              <label class="field"
                >Selesai
                <input
                  v-model="endDate"
                  type="date"
                  class="input"
                  :class="{ invalid: errors.end_date }"
                  data-testid="p-end"
                />
                <span v-if="errors.end_date" class="err">{{ errors.end_date }}</span>
              </label>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <label class="field"
                >Bobot KPI (%)
                <input
                  :value="kpiWeight"
                  type="number"
                  step="any"
                  min="0"
                  max="100"
                  class="input"
                  :class="{ invalid: errors.kpi_weight }"
                  data-testid="p-kpi"
                  @input="onKpi"
                />
                <span v-if="errors.kpi_weight" class="err">{{ errors.kpi_weight }}</span>
              </label>
              <label class="field"
                >Bobot perilaku (%)
                <input
                  :value="behaviorWeight"
                  type="number"
                  step="any"
                  min="0"
                  max="100"
                  class="input"
                  :class="{ invalid: errors.behavior_weight }"
                  data-testid="p-beh"
                  @input="onBehavior"
                />
                <span v-if="errors.behavior_weight" class="err">{{ errors.behavior_weight }}</span>
              </label>
            </div>
            <div class="flex justify-end gap-2.5">
              <button v-if="editingId" type="button" class="btn" @click="newForm">Batal ubah</button>
              <button type="submit" class="btn pri" :disabled="busy" data-testid="p-save">
                {{ editingId ? 'Simpan perubahan' : 'Simpan sebagai draf' }}
              </button>
            </div>
          </form>
        </section>
      </div>
    </template>
  </PageState>

  <Modal :open="!!confirm" :title="confirm ? ACT_TEXT[confirm.act].title : ''" @close="confirm = null">
    <p v-if="confirm" class="mt-0">
      <b>{{ confirm.period.name }}</b> — {{ ACT_TEXT[confirm.act].body }}
    </p>
    <template #footer>
      <button type="button" class="btn" @click="confirm = null">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="confirm-act" @click="runAct">
        {{ confirm ? ACT_TEXT[confirm.act].btn : '' }}
      </button>
    </template>
  </Modal>
</template>
