<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import KpiTable from '@/components/kpi/KpiTable.vue'
import Icon from '@/components/ui/Icon.vue'
import { ApiError, kpi, type KpiRow } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { achievement } from '@/lib/scoring'
import { DIRECTION_LABEL, fmtNum, fmtScore, RESULT_STATUS, fmtDate } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { usePeriodStore } from '@/stores/period'
import { useToastStore } from '@/stores/toast'

// H-05 — KPI Saya: tabel indikator + form input realisasi dengan bukti.
const auth = useAuthStore()
const period = usePeriodStore()
const toast = useToastStore()

const {
  data: sheet,
  loading,
  error,
  reload,
} = useLoader(async () => {
  await period.load()
  return kpi.sheet({ employee_id: auth.user?.id, period_id: period.selectedId ?? undefined })
}, [() => period.selectedId])

const rows = computed(() => sheet.value?.rows ?? [])
const selectedId = ref<number | null>(null)
const selected = computed(() => rows.value.find((r) => r.indicator.id === selectedId.value) ?? null)

// pilih baris pertama yang perlu diisi saat data (periode) berganti
watch(sheet, (s) => {
  const list = s?.rows ?? []
  if (!list.some((r) => r.indicator.id === selectedId.value)) {
    selectedId.value =
      (list.find((r) => r.status === 'missing' || r.status === 'rejected') ?? list[0])?.indicator.id ?? null
  }
})

// <input type=number> + v-model menghasilkan number (atau '' bila kosong), jadi selalu dinormalkan lewat String()
const actual = ref<string | number>('')
const note = ref('')
const file = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const busy = ref(false)
const errs = ref<{ actual?: string; file?: string; form?: string }>({})

function fill(r: KpiRow | null) {
  actual.value = r?.result ? String(Number(r.result.actual)) : ''
  note.value = r?.result?.note ?? ''
  file.value = null
  if (fileInput.value) fileInput.value.value = ''
  errs.value = {}
}
watch(selected, fill, { immediate: true })

const MAX = 5 * 1024 * 1024
const OK_TYPES = ['application/pdf', 'image/jpeg', 'image/png']

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0] ?? null
  errs.value.file = undefined
  if (f && !OK_TYPES.includes(f.type)) {
    errs.value.file = 'Hanya PDF, JPG, atau PNG'
    file.value = null
    return
  }
  if (f && f.size > MAX) {
    errs.value.file = 'Ukuran berkas maksimal 5 MB'
    file.value = null
    return
  }
  file.value = f
}

const preview = computed(() => {
  const r = selected.value
  const n = Number(actual.value)
  if (!r || String(actual.value) === '' || Number.isNaN(n) || n < 0) return null
  return achievement(n, Number(r.indicator.target), r.indicator.direction)
})

const editable = computed(() => !!sheet.value?.can_input && selected.value?.indicator.source === 'manual')

async function submit() {
  const r = selected.value
  const s = sheet.value
  if (!r || !s) return
  errs.value = {}
  const n = Number(actual.value)
  if (String(actual.value).trim() === '' || Number.isNaN(n) || n < 0) {
    errs.value.actual = 'Isi angka realisasi (tidak boleh negatif)'
    return
  }
  busy.value = true
  try {
    const saved = await kpi.saveResult({
      period_id: s.period.id,
      employee_id: s.employee.id,
      indicator_id: r.indicator.id,
      actual: n,
      note: note.value,
    })
    if (file.value) await kpi.uploadEvidence(saved.id, file.value)
    toast.success('Realisasi tersimpan dan dikirim ke atasan untuk divalidasi.')
    await reload()
  } catch (e) {
    if (e instanceof ApiError) {
      errs.value.actual = e.fields.actual
      errs.value.file = e.fields.file
      errs.value.form = e.message
    } else {
      errs.value.form = 'Gagal menyimpan'
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="sheet">
      <p class="hint -mb-3 mt-0">
        Periode {{ sheet.period.name }}
        <template v-if="sheet.period.status === 'active'"
          >(aktif, ditutup {{ fmtDate(sheet.period.end_date) }})</template
        >
        <template v-else-if="sheet.period.status === 'closed'">(sudah ditutup — data terkunci)</template>
      </p>

      <div
        v-if="sheet.period.status !== 'active'"
        class="rounded-xl bg-info-bg px-4 py-3 text-info-fg"
        role="status"
      >
        Periode ini {{ sheet.period.status === 'closed' ? 'sudah ditutup' : 'belum dibuka' }}; realisasi tidak
        dapat diubah.
      </div>
      <div v-else-if="!sheet.can_input" class="rounded-xl bg-info-bg px-4 py-3 text-info-fg" role="status">
        Penilaian Anda untuk periode ini sudah final; realisasi terkunci.
      </div>

      <div class="grid grid-cols-[repeat(auto-fit,minmax(230px,1fr))] gap-4">
        <div class="card" data-testid="score-card">
          <div class="k">Skor KPI sementara</div>
          <div class="v">{{ fmtScore(sheet.score) }}</div>
          <div class="d">dari {{ sheet.total }} indikator berbobot</div>
        </div>
        <div class="card" data-testid="validated-card">
          <div class="k">Sudah divalidasi atasan</div>
          <div class="v">
            {{ sheet.validated }}
            <span class="text-base font-semibold text-ink-soft">/ {{ sheet.total }}</span>
          </div>
          <div class="d">
            {{ sheet.pending }} menunggu validasi<template v-if="sheet.rejected">
              · {{ sheet.rejected }} ditolak</template
            ><template v-if="sheet.missing"> · {{ sheet.missing }} belum diisi</template>
          </div>
        </div>
        <div class="card">
          <div class="k">Porsi skor akhir</div>
          <div class="v">{{ fmtNum(sheet.period.kpi_weight) }}%</div>
          <div class="d">{{ fmtNum(sheet.period.behavior_weight) }}% dari penilaian perilaku</div>
        </div>
      </div>

      <p v-if="!rows.length" class="card muted m-0">
        Departemen Anda belum memiliki template KPI aktif. Hubungi HR.
      </p>

      <div v-else class="flex flex-wrap items-start gap-6">
        <div class="min-w-0 flex-[2_1_780px]">
          <KpiTable
            :rows="rows"
            :score="sheet.score"
            :can-input="sheet.can_input"
            :selected-id="selectedId"
            :title="`Indikator ${sheet.period.name}`"
            hint="Capaian dihitung otomatis, maksimum 120"
            @select="(r) => (selectedId = r.indicator.id)"
          />
        </div>

        <section v-if="selected" class="card min-w-0 flex-[1_1_340px]" data-testid="input-card">
          <div class="mb-4 flex flex-wrap items-start gap-3">
            <div class="flex-1">
              <h2>{{ editable ? 'Input realisasi' : 'Detail realisasi' }}</h2>
              <p class="hint">{{ selected.indicator.name }}</p>
            </div>
            <span class="badge" :class="RESULT_STATUS[selected.status]?.cls">{{
              RESULT_STATUS[selected.status]?.label
            }}</span>
          </div>

          <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
            <div v-if="errs.form" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">
              {{ errs.form }}
            </div>
            <div
              v-if="selected.indicator.source === 'integration'"
              class="rounded-xl bg-info-bg px-4 py-3 text-info-fg"
            >
              Indikator ini diisi otomatis dari sistem terintegrasi.
            </div>

            <label class="field">
              Realisasi<template v-if="selected.indicator.unit"> ({{ selected.indicator.unit }})</template>
              <input
                v-model="actual"
                type="number"
                inputmode="decimal"
                step="any"
                min="0"
                class="input"
                :class="{ invalid: errs.actual }"
                :disabled="!editable || busy"
                data-testid="actual-input"
              />
              <small>
                Target
                {{ DIRECTION_LABEL[selected.indicator.direction] === 'lebih rendah lebih baik' ? '≤ ' : ''
                }}{{ fmtNum(selected.indicator.target) }}
                <template v-if="preview !== null">
                  · capaian dihitung <b data-testid="preview">{{ fmtScore(preview) }}</b></template
                >
              </small>
              <span v-if="errs.actual" class="err">{{ errs.actual }}</span>
            </label>

            <label class="field">
              Keterangan
              <textarea
                v-model="note"
                class="input"
                maxlength="2000"
                :disabled="!editable || busy"
                placeholder="Rincian kegiatan atau sumber data"
              />
            </label>

            <div class="field">
              Bukti pendukung
              <div
                v-if="selected.result?.evidence_name"
                class="flex items-center gap-2.5 rounded-[10px] bg-canvas px-3 py-2.5 text-[13px] font-normal"
              >
                <Icon name="file" :size="18" />
                <span class="min-w-0 flex-1 truncate">{{ selected.result.evidence_name }}</span>
                <a
                  :href="kpi.evidenceUrl(selected.result.id)"
                  target="_blank"
                  rel="noopener"
                  class="font-semibold"
                  >Lihat</a
                >
                <a :href="kpi.evidenceUrl(selected.result.id, true)" class="font-semibold">Unduh</a>
              </div>
              <p v-else-if="!editable" class="muted m-0 font-normal">Tidak ada bukti diunggah.</p>
              <label
                v-if="editable"
                class="flex cursor-pointer items-center gap-2.5 rounded-xl border-[1.5px] border-dashed border-line-input p-3.5 text-[13px] font-normal text-ink-soft"
              >
                <Icon name="upload" :size="20" />
                <span class="min-w-0 flex-1 truncate"
                  >{{ file ? file.name : selected.result?.evidence_name ? 'Ganti file' : 'Tambah file' }} ·
                  PDF, JPG, PNG, maks. 5 MB</span
                >
                <input
                  ref="fileInput"
                  type="file"
                  accept=".pdf,.jpg,.jpeg,.png,application/pdf,image/jpeg,image/png"
                  class="sr-only"
                  :disabled="busy"
                  @change="onFile"
                />
              </label>
              <span v-if="errs.file" class="err">{{ errs.file }}</span>
              <small v-if="editable && !selected.result && file"
                >Berkas diunggah setelah realisasi disimpan.</small
              >
            </div>

            <div v-if="editable" class="flex flex-wrap justify-end gap-2.5">
              <button type="button" class="btn" :disabled="busy" @click="fill(selected)">Batal</button>
              <button type="submit" class="btn pri" :disabled="busy" data-testid="save">
                {{ busy ? 'Menyimpan…' : 'Simpan & kirim ke atasan' }}
              </button>
            </div>
          </form>
        </section>
      </div>
    </template>
  </PageState>
</template>
