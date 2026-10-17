<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import BalanceList from '@/components/leave/BalanceList.vue'
import { ApiError, leave, type Colleague, type LeaveBalance, type LeavePreview, type LeaveType } from '@/api'
import { useToastStore } from '@/stores/toast'
import { useLoader } from '@/lib/useLoader'
import { fmtDays, fmtHours, fmtWeekdayDate, fromMinutes, parseHours, toMinutes, todayYmd } from '@/lib/leave'

// H-08 — Pengajuan Izin & Cuti (juga tampilan ponsel M08). Jumlah hari kerja dihitung API secara live.
const router = useRouter()
const toast = useToastStore()

const MAX_FILE = 5 * 1024 * 1024
const FILE_TYPES = ['application/pdf', 'image/jpeg', 'image/png']

const { data, loading, error, reload } = useLoader(async () => {
  const [types, balances, subs] = await Promise.all([leave.types(), leave.balances(), leave.substitutes()])
  return { types: types.filter((t) => t.is_active), balances, subs }
})

function addDays(ymd: string, n: number): string {
  const [y, m, d] = ymd.split('-').map(Number) as [number, number, number]
  const t = new Date(Date.UTC(y, m - 1, d + n))
  return t.toISOString().slice(0, 10)
}
/** Hari kerja berikutnya (Senin–Jumat) mulai besok; hari libur nasional dikoreksi oleh perhitungan server. */
function nextWeekday(): string {
  let d = addDays(todayYmd(), 1)
  for (;;) {
    const [y, m, dd] = d.split('-').map(Number) as [number, number, number]
    const dow = new Date(Date.UTC(y, m - 1, dd)).getUTCDay()
    if (dow !== 0 && dow !== 6) return d
    d = addDays(d, 1)
  }
}

const typeId = ref<number | null>(null)
const start = ref(nextWeekday())
const end = ref(start.value)
const mode = ref<'full' | 'partial'>('full')
const fromTime = ref('')
const toTime = ref('')
const reason = ref('')
const substituteId = ref<number | null>(null)
const file = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const errors = ref<Record<string, string>>({})
const busy = ref(false)

const half = computed(() => mode.value === 'partial')
const type = computed<LeaveType | null>(() => data.value?.types.find((t) => t.id === typeId.value) ?? null)
const minDate = computed(() => addDays(todayYmd(), -7))

watch(
  data,
  (d) => {
    if (d && typeId.value === null) typeId.value = d.types[0]?.id ?? null
  },
  { immediate: true },
)
// jenis yang tidak mengizinkan setengah hari → kembali ke sehari penuh
watch(type, (t) => {
  if (t && !t.allow_half_day) mode.value = 'full'
})
watch(half, (h) => {
  if (h) end.value = start.value
})
watch(start, (s) => {
  if (half.value || end.value < s) end.value = s
})

// ---- pratinjau live ----
const preview = ref<LeavePreview | null>(null)
const previewError = ref('')
const previewing = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined
let seq = 0

async function runPreview() {
  if (!start.value || !end.value) return
  if (end.value < start.value) {
    preview.value = null
    previewError.value = 'Tanggal selesai tidak boleh sebelum tanggal mulai.'
    return
  }
  const my = ++seq
  previewing.value = true
  try {
    const r = await leave.preview({
      type_id: typeId.value ?? undefined,
      start_date: start.value,
      end_date: end.value,
      half_day: half.value,
      ...(half.value && fromTime.value && toTime.value
        ? { start_time: fromTime.value, end_time: toTime.value }
        : {}),
    })
    if (my !== seq) return
    preview.value = r
    previewError.value = ''
  } catch (e) {
    if (my !== seq) return
    preview.value = null
    previewError.value = e instanceof ApiError ? e.message : 'Gagal menghitung hari kerja.'
  } finally {
    if (my === seq) previewing.value = false
  }
}
watch([typeId, start, end, mode, fromTime, toTime], () => {
  clearTimeout(timer)
  timer = setTimeout(runPreview, 250)
})
onMounted(runPreview)
onBeforeUnmount(() => clearTimeout(timer))

// jam kerja hari yang dipilih (dari rincian API) — dasar tombol Pagi/Siang dan jam bawaan
const dayHours = computed(() =>
  parseHours((preview.value?.working_days.breakdown ?? []).find((d) => d.counted)?.hours),
)
function quick(which: 'pagi' | 'siang') {
  // paruh pertama/kedua dihitung server dari jam efektif (istirahat tidak ikut dihitung)
  const w = preview.value?.working_days
  const r = parseHours(which === 'pagi' ? w?.morning : w?.afternoon)
  if (!r) return
  fromTime.value = fromMinutes(r[0])
  toTime.value = fromMinutes(r[1])
}
// masuk mode sebagian hari: isi paruh pagi begitu jam kerja hari itu diketahui
watch([preview, mode], () => {
  if (half.value && !fromTime.value && dayHours.value) quick('pagi')
})
watch(mode, (m) => {
  if (m === 'full') fromTime.value = toTime.value = ''
})
// ganti tanggal: jam lama bisa di luar jam kerja hari baru
watch(start, () => {
  fromTime.value = toTime.value = ''
})

const partialInfo = computed(() => {
  const w = preview.value?.working_days
  if (!half.value || !w?.partial_minutes || !w.day_minutes) return ''
  return `${fmtHours(w.partial_minutes)} dari ${fmtHours(w.day_minutes)} kerja efektif hari itu`
})
// ringkasan jam kerja per hari pada rentang, mis. "Sen–Jum 09:00–17:00 · Sab 09:00–15:00"
const hoursSummary = computed(() => {
  const seen = new Map<string, string>()
  for (const d of preview.value?.working_days.breakdown ?? []) {
    if (!d.counted || !d.hours) continue
    seen.set(
      d.hours,
      [...(seen.get(d.hours)?.split(', ') ?? []), d.weekday]
        .filter((x, i, a) => a.indexOf(x) === i)
        .join(', '),
    )
  }
  return [...seen.entries()].map(([h, days]) => `${days} ${h}`).join(' · ')
})

const days = computed(() => (preview.value ? Number(preview.value.working_days.days) : 0))
const needFile = computed(
  () => !!type.value?.requires_attachment && days.value > Number(type.value.attachment_min_days),
)

const balanceFor = (id: number | null): LeaveBalance | undefined =>
  data.value?.balances.find((b) => b.leave_type.id === id)

function subText(t: LeaveType): string {
  const b = balanceFor(t.id)
  if (b?.tracked) return `sisa ${fmtDays(b.available)} hari`
  if (t.allow_half_day) return 'setengah / satu hari'
  if (t.requires_attachment) {
    return Number(t.attachment_min_days) > 0
      ? `lampiran jika > ${fmtDays(t.attachment_min_days)} hari`
      : 'lampiran wajib'
  }
  if (t.max_days_per_request) return `maks ${fmtDays(t.max_days_per_request)} hari`
  return 'dicatat saja'
}

const annualBalance = computed(() => balanceFor(data.value?.types.find((t) => t.code === 'CT')?.id ?? null))
const days0 = computed(() => preview.value?.working_days.breakdown ?? [])
const holidayNote = computed(() => {
  const h = (preview.value?.working_days.skipped ?? []).filter((d) => d.skip === 'holiday')
  return h.length ? h.map((d) => d.holiday).join(', ') : ''
})
const dayLabel = (iso: string) => {
  const d = days0.value.find((x) => x.date === iso)
  return d ? `${d.weekday} ${Number(iso.slice(8, 10))}` : iso
}

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0] ?? null
  errors.value = { ...errors.value, attachment: '' }
  if (f && f.size > MAX_FILE) {
    errors.value.attachment = 'Ukuran berkas maksimal 5 MB.'
    file.value = null
    if (fileInput.value) fileInput.value.value = ''
    return
  }
  if (f && f.type && !FILE_TYPES.includes(f.type)) {
    errors.value.attachment = 'Hanya PDF, JPG, atau PNG.'
    file.value = null
    if (fileInput.value) fileInput.value.value = ''
    return
  }
  file.value = f
}

async function submit() {
  errors.value = {}
  const e: Record<string, string> = {}
  if (!typeId.value) e.leave_type_id = 'Pilih jenis pengajuan'
  if (!start.value) e.start_date = 'Tanggal mulai wajib diisi'
  if (!end.value) e.end_date = 'Tanggal selesai wajib diisi'
  if (!reason.value.trim()) e.reason = 'Alasan wajib diisi'
  if (half.value && (!fromTime.value || !toTime.value)) e.end_time = 'Isi jam mulai dan jam selesai'
  else if (half.value && toMinutes(toTime.value) <= toMinutes(fromTime.value))
    e.end_time = 'Jam selesai harus setelah jam mulai'
  if (!previewError.value && preview.value && days.value <= 0)
    e.start_date = 'Rentang ini tidak berisi hari kerja (akhir pekan atau hari libur)'
  if (previewError.value) e.end_date = previewError.value
  if (needFile.value && !file.value) e.attachment = `Lampiran wajib diunggah untuk ${type.value?.name}`
  if (Object.keys(e).length) {
    errors.value = e
    return
  }
  busy.value = true
  try {
    const r = await leave.submit({
      leave_type_id: typeId.value!,
      start_date: start.value,
      end_date: end.value,
      half_day: half.value,
      half_session: 'pagi',
      start_time: fromTime.value,
      end_time: toTime.value,
      reason: reason.value.trim(),
      substitute_id: substituteId.value,
      attachment: file.value,
    })
    toast.success(`Pengajuan ${r.number ?? ''} terkirim.`)
    await router.push({ path: '/izin-cuti/riwayat', query: { id: String(r.id) } })
  } catch (err) {
    if (err instanceof ApiError) {
      errors.value = { ...err.fields, form: err.message }
      if (err.status !== 422) toast.error(err.message)
    } else {
      toast.error('Gagal mengirim pengajuan.')
    }
  } finally {
    busy.value = false
  }
}

const subs = computed<Colleague[]>(() => data.value?.subs ?? [])
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="data">
      <div>
        <h2 class="m-0 text-xl font-bold">Ajukan izin atau cuti</h2>
        <p class="hint">
          Jumlah hari dihitung otomatis tanpa akhir pekan dan hari libur
          <template v-if="annualBalance?.available != null">
            <span class="lg:hidden"> · sisa cuti tahunan {{ fmtDays(annualBalance.available) }} hari</span>
          </template>
        </p>
      </div>

      <div class="flex flex-wrap items-start gap-6">
        <form
          class="card flex min-w-0 flex-[2_1_520px] flex-col gap-5 sm:gap-[22px]"
          novalidate
          data-testid="leave-form"
          @submit.prevent="submit"
        >
          <div
            v-if="errors.form && !Object.keys(errors).some((k) => k !== 'form')"
            class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg"
            role="alert"
          >
            {{ errors.form }}
          </div>

          <div class="field" role="radiogroup" aria-label="Jenis pengajuan">
            Jenis pengajuan
            <div
              class="flex flex-wrap gap-2 sm:grid sm:grid-cols-[repeat(auto-fill,minmax(150px,1fr))] sm:gap-2.5"
            >
              <button
                v-for="t in data.types"
                :key="t.id"
                type="button"
                role="radio"
                :aria-checked="typeId === t.id"
                :data-testid="`type-${t.code}`"
                class="inline-flex min-h-[40px] items-center rounded-full border px-3.5 text-[13px] font-semibold transition sm:min-h-[64px] sm:flex-col sm:items-start sm:justify-center sm:rounded-xl sm:px-3.5 sm:py-2.5"
                :class="
                  typeId === t.id
                    ? 'border-primary bg-primary text-white sm:bg-primary-tint sm:text-ink sm:ring-1 sm:ring-primary'
                    : 'border-line-input bg-white text-ink hover:bg-canvas'
                "
                @click="typeId = t.id"
              >
                <b class="font-bold">{{ t.name }}</b>
                <span class="hidden text-xs font-normal text-ink-soft sm:block">{{ subText(t) }}</span>
              </button>
            </div>
            <span v-if="errors.leave_type_id" class="err">{{ errors.leave_type_id }}</span>
          </div>

          <div v-if="type?.allow_half_day" class="field" role="radiogroup" aria-label="Durasi">
            Durasi
            <div class="flex flex-wrap gap-2">
              <button
                v-for="o in [
                  { v: 'full', l: 'Satu hari / lebih' },
                  { v: 'partial', l: 'Sebagian hari (per jam)' },
                ]"
                :key="o.v"
                type="button"
                role="radio"
                :aria-checked="mode === o.v"
                :data-testid="`mode-${o.v}`"
                class="inline-flex min-h-[40px] items-center rounded-full border px-3.5 text-[13px] font-semibold"
                :class="
                  mode === o.v
                    ? 'border-primary bg-primary text-white'
                    : 'border-line-input bg-white text-ink hover:bg-canvas'
                "
                @click="mode = o.v as typeof mode"
              >
                {{ o.l }}
              </button>
            </div>
            <div v-if="half" class="mt-1 grid grid-cols-2 gap-3 sm:gap-4" data-testid="partial-box">
              <label class="field"
                >Dari jam
                <input
                  v-model="fromTime"
                  type="time"
                  step="900"
                  class="input"
                  :class="{ invalid: errors.start_time || errors.end_time }"
                  data-testid="from"
                />
                <span v-if="errors.start_time" class="err">{{ errors.start_time }}</span>
              </label>
              <label class="field"
                >Sampai jam
                <input
                  v-model="toTime"
                  type="time"
                  step="900"
                  class="input"
                  :class="{ invalid: errors.end_time }"
                  data-testid="to"
                />
                <span v-if="errors.end_time" class="err">{{ errors.end_time }}</span>
              </label>
              <div class="col-span-2 flex flex-wrap gap-2">
                <button type="button" class="btn sm" data-testid="quick-pagi" @click="quick('pagi')">
                  Paruh pagi
                </button>
                <button type="button" class="btn sm" data-testid="quick-siang" @click="quick('siang')">
                  Paruh siang
                </button>
                <span v-if="dayHours" class="muted self-center text-xs"
                  >Jam kerja hari itu {{ fromMinutes(dayHours[0]) }}–{{ fromMinutes(dayHours[1])
                  }}<template v-if="preview?.working_days.break">
                    · istirahat {{ preview.working_days.break }}</template
                  ></span
                >
              </div>
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3 sm:gap-4">
            <label class="field"
              >Tanggal mulai
              <input
                v-model="start"
                type="date"
                class="input"
                :class="{ invalid: errors.start_date }"
                :min="minDate"
                data-testid="start"
              />
              <small>{{ fmtWeekdayDate(start) }}</small>
              <span v-if="errors.start_date" class="err">{{ errors.start_date }}</span>
            </label>
            <label class="field"
              >Tanggal selesai
              <input
                v-model="end"
                type="date"
                class="input"
                :class="{ invalid: errors.end_date }"
                :min="start"
                :disabled="half"
                data-testid="end"
              />
              <small>{{ fmtWeekdayDate(end) }}</small>
              <span v-if="errors.end_date" class="err">{{ errors.end_date }}</span>
            </label>
          </div>

          <div
            class="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-[14px] border border-line bg-canvas/60 p-3.5 sm:p-4"
            data-testid="calc"
            aria-live="polite"
          >
            <div class="flex items-center gap-3">
              <div
                class="grid h-12 w-12 place-items-center rounded-xl bg-primary-soft text-xl font-extrabold text-primary"
                data-testid="days"
              >
                {{ preview ? fmtDays(preview.working_days.days) : '–' }}
              </div>
              <div>
                <b>hari kerja</b>
                <div class="muted text-xs">
                  <template v-if="previewing">menghitung…</template>
                  <template v-else-if="previewError"
                    ><span class="text-bad-fg">{{ previewError }}</span></template
                  >
                  <template v-else-if="preview && partialInfo">{{ partialInfo }}</template>
                  <template v-else-if="preview"
                    >{{ fmtWeekdayDate(start).split(',')[0] }} – {{ fmtWeekdayDate(end).split(',')[0] }} ·
                    {{ preview.working_days.calendar_days }} hari kalender</template
                  >
                </div>
              </div>
            </div>
            <div v-if="preview" class="flex flex-wrap gap-1.5" aria-label="Rincian hari">
              <span
                v-for="d in (preview.working_days.breakdown ?? []).slice(0, 31)"
                :key="d.date"
                class="rounded-md px-2 py-0.5 text-xs font-semibold"
                :class="
                  d.counted ? 'bg-primary-soft text-[#0B4A43]' : 'bg-line-soft text-ink-soft line-through'
                "
                :title="d.holiday || (d.skip === 'weekend' ? 'Bukan hari kerja' : d.hours)"
                >{{ dayLabel(d.date) }}</span
              >
            </div>
            <p v-if="hoursSummary" class="muted m-0 basis-full text-[12.5px]" data-testid="hours">
              Jam kerja ({{ preview?.working_days.schedule }}): {{ hoursSummary
              }}<template v-if="preview?.working_days.break">
                · istirahat {{ preview.working_days.break }} tidak dihitung</template
              >
            </p>
            <p class="muted m-0 basis-full text-[12.5px]">
              <template v-if="preview && !(preview.working_days.skipped ?? []).length"
                >Tidak ada hari libur atau hari di luar jadwal kerja pada rentang ini.</template
              >
              <template v-else-if="preview"
                >Tidak dihitung: {{ (preview.working_days.skipped ?? []).length }} hari<template
                  v-if="holidayNote"
                >
                  (libur: {{ holidayNote }})</template
                >.</template
              >
            </p>
            <ul v-if="preview?.warnings?.length" class="m-0 basis-full list-none p-0" role="alert">
              <li v-for="w in preview.warnings ?? []" :key="w" class="text-[13px] font-semibold text-warn-fg">
                ⚠ {{ w }}
              </li>
            </ul>
          </div>

          <label class="field"
            >Alasan
            <textarea
              v-model="reason"
              class="input"
              :class="{ invalid: errors.reason }"
              maxlength="1000"
              placeholder="Contoh: acara keluarga di Yogyakarta."
              data-testid="reason"
            />
            <span v-if="errors.reason" class="err">{{ errors.reason }}</span>
          </label>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="field"
              >Karyawan pengganti <small>opsional</small>
              <select v-model="substituteId" class="input" :class="{ invalid: errors.substitute_id }">
                <option :value="null">— tanpa pengganti —</option>
                <option v-for="s in subs" :key="s.id" :value="s.id">{{ s.name }}</option>
              </select>
              <span v-if="errors.substitute_id" class="err">{{ errors.substitute_id }}</span>
            </label>
            <label class="field"
              >Lampiran
              <small
                >{{ needFile ? 'wajib untuk pengajuan ini' : 'opsional' }} · PDF, JPG, PNG maks 5 MB</small
              >
              <input
                ref="fileInput"
                type="file"
                accept=".pdf,.jpg,.jpeg,.png,application/pdf,image/jpeg,image/png"
                class="input !py-2 file:mr-3 file:rounded-lg file:border-0 file:bg-primary-soft file:px-3 file:py-1.5 file:font-semibold file:text-primary"
                :class="{ invalid: errors.attachment }"
                data-testid="file"
                @change="onFile"
              />
              <span v-if="errors.attachment" class="err">{{ errors.attachment }}</span>
            </label>
          </div>

          <div class="flex flex-wrap justify-end gap-2.5 border-t border-line-soft pt-[18px]">
            <RouterLink to="/dashboard" class="btn max-sm:flex-1">Batal</RouterLink>
            <button
              type="submit"
              class="btn pri min-h-[50px] max-sm:flex-[2] sm:min-h-[44px]"
              :disabled="busy"
              data-testid="submit"
            >
              {{ busy ? 'Mengirim…' : 'Kirim pengajuan' }}
            </button>
          </div>
        </form>

        <div class="flex min-w-0 flex-[1_1_320px] flex-col gap-6">
          <section class="card">
            <h2>Saldo {{ new Date().getFullYear() }}</h2>
            <p class="hint mb-3.5">Diperbarui saat pengajuan disetujui</p>
            <BalanceList :balances="data.balances" :extra="days" :extra-type-id="typeId" />
          </section>

          <section class="card" data-testid="flow">
            <h2>Alur persetujuan</h2>
            <ol class="m-0 mt-3.5 flex list-none flex-col gap-3.5 p-0">
              <li v-for="s in preview?.flow ?? []" :key="s.level" class="flex gap-3">
                <span
                  class="grid h-7 w-7 flex-none place-items-center rounded-full bg-primary-soft text-[13px] font-bold text-primary"
                  >{{ s.level }}</span
                >
                <div>
                  <b>{{ s.names.join(', ') || '—' }}</b>
                  <div class="muted text-[12.5px]">
                    {{ s.role
                    }}<template v-if="s.level === 2 && s.role === 'HR'">
                      · saldo dipotong setelah disetujui</template
                    >
                  </div>
                </div>
              </li>
            </ol>
            <p class="hint mb-0 mt-3.5">Anda akan menerima email di setiap perubahan status.</p>
          </section>
        </div>
      </div>
    </template>
  </PageState>
</template>
