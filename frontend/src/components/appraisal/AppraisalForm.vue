<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import AspectScale from './AspectScale.vue'
import { behaviorScore, finalScore, gradeOf } from '@/lib/scoring'
import { fmtNum, fmtScore, gradeClass, RESULT_STATUS } from '@/lib/format'
import type { AppraisalView } from '@/api'

// Form penilaian (H-07). Karyawan mengisi self-assessment; atasan menilai aspek yang sama.
// Skor akhir dihitung ulang di sini hanya sebagai pratinjau; angka resmi dari backend.
const props = defineProps<{ view: AppraisalView; busy?: boolean }>()
const emit = defineEmits<{
  save: [payload: { scores: Record<number, number>; comment: string; managerComment: string; developmentPlan: string }, submit: boolean]
  finalize: []
}>()

type Mode = 'manager' | 'self' | 'readonly'
const mode = computed<Mode>(() => (props.view.can_manager ? 'manager' : props.view.can_self ? 'self' : 'readonly'))
const aspects = computed(() => props.view.aspects ?? [])

const scores = reactive<Record<number, number | null>>({})
const text = reactive({ comment: '', managerComment: '', developmentPlan: '' })

function reset() {
  for (const a of aspects.value) {
    scores[a.aspect.id] = (mode.value === 'self' ? a.self_score : a.manager_score) ?? null
  }
  text.comment = props.view.employee_comment ?? ''
  text.managerComment = props.view.manager_comment ?? ''
  text.developmentPlan = props.view.development_plan ?? ''
}
watch(() => props.view, reset, { immediate: true })

const editable = computed(() => mode.value !== 'readonly')
const answered = computed(() => aspects.value.filter((a) => scores[a.aspect.id] != null).length)
const complete = computed(() => aspects.value.length > 0 && answered.value === aspects.value.length)

const kpi = computed(() => Number(props.view.kpi_score))
const kw = computed(() => Number(props.view.period.kpi_weight))
const bw = computed(() => Number(props.view.period.behavior_weight))

// Pratinjau: nilai atasan yang sedang diisi (mode manager) atau nilai tersimpan dari backend.
const managerValues = computed(() =>
  mode.value === 'manager'
    ? aspects.value.map((a) => scores[a.aspect.id]).filter((v): v is number => v != null)
    : aspects.value.map((a) => a.manager_score).filter((v): v is number => v != null),
)
const behavior = computed<number | null>(() => {
  if (aspects.value.length > 0 && managerValues.value.length === aspects.value.length) return behaviorScore(managerValues.value)
  return props.view.behavior_score != null ? Number(props.view.behavior_score) : null
})
const average = computed(() =>
  managerValues.value.length ? managerValues.value.reduce((a, b) => a + b, 0) / managerValues.value.length : null,
)
const final = computed<number | null>(() => (behavior.value != null ? finalScore(kpi.value, behavior.value, kw.value, bw.value) : null))
const grade = computed(() => (final.value != null ? gradeOf(final.value) : ''))
const showScore = computed(() => !props.view.hidden_from_employee)

function payload() {
  const s: Record<number, number> = {}
  for (const a of aspects.value) {
    const v = scores[a.aspect.id]
    if (v != null) s[a.aspect.id] = v
  }
  return { scores: s, comment: text.comment, managerComment: text.managerComment, developmentPlan: text.developmentPlan }
}
</script>

<template>
  <div class="flex flex-wrap items-start gap-6">
    <div class="flex min-w-0 flex-[2_1_620px] flex-col gap-6">
      <section class="card">
        <div class="mb-4">
          <h2>Aspek perilaku</h2>
          <p class="hint">
            1 = sangat kurang · 3 = sesuai harapan · 5 = jauh melampaui.
            <template v-if="mode === 'manager'">Nilai karyawan ditampilkan sebagai pembanding.</template>
            <template v-else-if="mode === 'self'">Nilai ini tidak masuk perhitungan skor akhir.</template>
          </p>
        </div>
        <p v-if="!aspects.length" class="muted m-0">Belum ada aspek perilaku.</p>
        <div
          v-for="a in aspects"
          :key="a.aspect.id"
          class="flex flex-wrap items-center gap-x-5 gap-y-3 border-b border-line-soft py-4 last:border-b-0"
          :data-testid="`aspect-${a.aspect.id}`"
        >
          <div class="min-w-[200px] flex-1">
            <b>{{ a.aspect.name }}</b>
            <div class="muted text-[12.5px]">{{ a.aspect.description }}</div>
          </div>
          <span v-if="mode !== 'self' && a.self_score != null" class="whitespace-nowrap rounded-lg bg-canvas px-2.5 py-1.5 text-xs text-ink-soft" data-testid="self-pill">
            Karyawan: {{ a.self_score }}
          </span>
          <AspectScale
            :model-value="scores[a.aspect.id] ?? null"
            :label="`Nilai ${a.aspect.name}`"
            :disabled="!editable || busy"
            @update:model-value="(v) => (scores[a.aspect.id] = v)"
          />
        </div>
      </section>

      <section class="card flex flex-col gap-4">
        <div>
          <h2>Catatan</h2>
          <p class="hint">Terlihat oleh karyawan setelah penilaian final</p>
        </div>

        <div v-if="mode !== 'self'" class="field">
          Catatan pencapaian dari karyawan
          <div class="rounded-[10px] bg-canvas px-3.5 py-3 font-normal" data-testid="employee-comment">
            {{ view.employee_comment || 'Belum ada catatan dari karyawan.' }}
          </div>
        </div>
        <label v-else class="field">
          Catatan pencapaian
          <textarea v-model="text.comment" class="input" :disabled="!editable || busy" maxlength="5000" data-testid="self-comment" />
        </label>

        <template v-if="mode === 'manager'">
          <label class="field">
            Komentar atasan
            <textarea v-model="text.managerComment" class="input" :disabled="busy" maxlength="5000" data-testid="manager-comment" />
          </label>
          <label class="field">
            Rencana pengembangan
            <textarea v-model="text.developmentPlan" class="input" :disabled="busy" maxlength="5000" data-testid="development-plan" />
          </label>
        </template>
        <template v-else-if="view.hidden_from_employee">
          <p class="m-0 rounded-[10px] bg-info-bg px-3.5 py-3 text-info-fg" data-testid="hidden-note">
            Nilai, komentar, dan rencana pengembangan dari atasan akan terlihat setelah penilaian difinalisasi.
          </p>
        </template>
        <template v-else>
          <div class="field">Komentar atasan<div class="rounded-[10px] bg-canvas px-3.5 py-3 font-normal">{{ view.manager_comment || '—' }}</div></div>
          <div class="field">Rencana pengembangan<div class="rounded-[10px] bg-canvas px-3.5 py-3 font-normal">{{ view.development_plan || '—' }}</div></div>
        </template>
      </section>
    </div>

    <div class="flex min-w-0 flex-[1_1_320px] flex-col gap-6">
      <section class="card">
        <div class="mb-4">
          <h2>Ringkasan KPI</h2>
          <p class="hint">Dari realisasi yang diinput</p>
        </div>
        <div v-for="r in view.kpi.rows ?? []" :key="r.indicator.id" class="flex justify-between gap-3 border-b border-dashed border-line py-2 text-[13.5px] last:border-b-0">
          <span>{{ r.indicator.name }}</span>
          <b class="text-right">
            {{ r.status === 'missing' ? '—' : fmtScore(r.achievement) }}
            <span v-if="r.status !== 'validated'" class="badge ml-1" :class="RESULT_STATUS[r.status]?.cls">
              {{ r.status === 'missing' ? 'belum diisi' : r.status === 'rejected' ? 'ditolak' : 'belum valid' }}
            </span>
          </b>
        </div>
      </section>

      <section class="card !border-2 !border-primary">
        <h2>Skor akhir</h2>
        <div class="mt-2 flex justify-between gap-3 py-2.5 text-sm">
          <span>KPI {{ fmtScore(kpi) }} × {{ fmtNum(kw) }}%</span>
          <b data-testid="kpi-part">{{ fmtScore((kpi * kw) / 100) }}</b>
        </div>
        <template v-if="showScore">
          <div class="flex justify-between gap-3 py-2.5 text-sm">
            <span>Perilaku {{ fmtScore(behavior) }} × {{ fmtNum(bw) }}%</span>
            <b data-testid="behavior-part">{{ behavior != null ? fmtScore((behavior * bw) / 100) : '—' }}</b>
          </div>
          <div class="-mt-1 text-xs text-ink-soft" data-testid="average">
            rata-rata nilai atasan {{ average != null ? fmtScore(average) : '—' }} × 20
          </div>
        </template>
        <div class="mt-2.5 flex items-baseline justify-between gap-3 border-t border-line pt-3.5">
          <b class="text-[40px] tracking-tight" data-testid="final-score">{{ showScore && final != null ? fmtScore(final) : '—' }}</b>
          <span v-if="showScore && grade" class="badge !px-3.5 !py-1.5 !text-sm" :class="gradeClass(grade)" data-testid="grade">{{ grade }}</span>
        </div>
        <p v-if="!showScore" class="hint mt-2">Skor akhir dan predikat tampil setelah penilaian final.</p>

        <div v-if="editable" class="mt-[18px] flex flex-col gap-2.5">
          <button
            type="button"
            class="btn pri"
            :disabled="busy || !complete"
            data-testid="submit"
            @click="emit('save', payload(), true)"
          >
            {{ mode === 'manager' ? 'Kirim penilaian' : 'Kirim self-assessment' }}
          </button>
          <button type="button" class="btn" :disabled="busy" data-testid="draft" @click="emit('save', payload(), false)">Simpan draf</button>
          <p v-if="!complete" class="hint m-0" data-testid="incomplete">Nilai semua aspek ({{ answered }}/{{ aspects.length }}) untuk mengirim.</p>
        </div>
        <div v-if="view.can_finalize" class="mt-3.5">
          <button type="button" class="btn pri w-full" :disabled="busy" data-testid="finalize" @click="emit('finalize')">Finalisasi hasil</button>
          <p class="hint mt-2">Final mengunci penilaian. Semua realisasi KPI harus sudah tervalidasi.</p>
        </div>
        <p v-else-if="mode === 'manager'" class="hint mt-2.5">Setelah dikirim, HR atau Direktur yang memfinalisasi.</p>
      </section>
    </div>
  </div>
</template>
