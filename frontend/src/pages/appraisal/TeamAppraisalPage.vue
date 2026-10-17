<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import Avatar from '@/components/ui/Avatar.vue'
import Modal from '@/components/ui/Modal.vue'
import { ApiError, appraisals, kpi, type Employee, type KpiRow, type KpiSheet } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { APPRAISAL_STATUS, fmtNum, fmtScore, gradeClass } from '@/lib/format'
import { usePeriodStore } from '@/stores/period'
import { useToastStore } from '@/stores/toast'

// H-06 — Penilaian Tim: validasi realisasi KPI + status penilaian anggota.
const period = usePeriodStore()
const toast = useToastStore()

interface Pending {
  key: string
  employee: Employee
  row: KpiRow
}

const { data, loading, error, reload } = useLoader(async () => {
  await period.load()
  const pid = period.selectedId
  if (!pid) throw new ApiError(404, 'not_found', 'Belum ada periode penilaian')
  const team = await appraisals.team(pid)
  // ambil lembar KPI hanya untuk anggota yang punya realisasi menunggu validasi
  const withPending = team.rows.filter((r) => r.kpi_pending > 0)
  const sheets: KpiSheet[] = await Promise.all(
    withPending.map((r) => kpi.sheet({ employee_id: r.employee.id, period_id: pid })),
  )
  const pending: Pending[] = sheets.flatMap((s) =>
    (s.rows ?? [])
      .filter((r) => r.status === 'submitted' && r.result)
      .map((r) => ({ key: `${s.employee.id}-${r.indicator.id}`, employee: s.employee, row: r })),
  )
  return {
    team,
    pending,
    canValidate: sheets.length ? sheets.every((s) => s.can_validate) : team.period.status === 'active',
  }
}, [() => period.selectedId])

const selected = ref<Set<string>>(new Set())
const busy = ref(false)
const pending = computed(() => data.value?.pending ?? [])
const allSelected = computed(() => pending.value.length > 0 && selected.value.size === pending.value.length)

function toggle(key: string) {
  const s = new Set(selected.value)
  if (!s.delete(key)) s.add(key)
  selected.value = s
}
function toggleAll() {
  selected.value = allSelected.value ? new Set() : new Set(pending.value.map((p) => p.key))
}

async function approve(items: Pending[]) {
  if (!items.length) return
  busy.value = true
  let ok = 0
  try {
    for (const it of items) {
      await kpi.validate(it.row.result!.id, 'validate')
      ok++
    }
    toast.success(`${ok} realisasi disetujui.`)
  } catch (e) {
    toast.error(`${ok} disetujui, lalu gagal: ${e instanceof ApiError ? e.message : 'kesalahan'}`)
  } finally {
    busy.value = false
    selected.value = new Set()
    await reload()
  }
}

// --- tolak / kembalikan dengan catatan ---
const rejecting = ref<Pending | null>(null)
const reason = ref('')
const reasonErr = ref('')

function askReject(p: Pending) {
  rejecting.value = p
  reason.value = ''
  reasonErr.value = ''
}

async function confirmReject() {
  if (!rejecting.value) return
  if (!reason.value.trim()) {
    reasonErr.value = 'Catatan wajib diisi agar karyawan tahu apa yang harus diperbaiki'
    return
  }
  busy.value = true
  try {
    await kpi.validate(rejecting.value.row.result!.id, 'reject', reason.value.trim())
    toast.success('Realisasi dikembalikan ke karyawan.')
    rejecting.value = null
    await reload()
  } catch (e) {
    reasonErr.value = e instanceof ApiError ? e.message : 'Gagal mengembalikan'
  } finally {
    busy.value = false
  }
}

function nextAction(status: string): string {
  return status === 'final'
    ? 'Lihat'
    : status === 'manager_done'
      ? 'Ubah nilai'
      : status === 'self_done'
        ? 'Mulai menilai'
        : 'Lihat / nilai'
}
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="data">
      <p class="hint -mb-3 mt-0">
        {{ data.team.period.name }} · realisasi KPI → self-assessment → penilaian atasan → final
      </p>

      <div
        v-if="data.team.period.status !== 'active'"
        class="rounded-xl bg-info-bg px-4 py-3 text-info-fg"
        role="status"
      >
        Periode ini {{ data.team.period.status === 'closed' ? 'sudah ditutup' : 'belum dibuka' }}; validasi
        dan penilaian tidak dapat diubah.
      </div>

      <div class="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
        <div class="card">
          <div class="k">Anggota</div>
          <div class="v">{{ data.team.summary.members }}</div>
        </div>
        <div class="card" data-testid="pending-count">
          <div class="k">Realisasi menunggu validasi</div>
          <div class="v">{{ data.team.summary.pending_kpi }}</div>
        </div>
        <div class="card">
          <div class="k">Sudah dinilai atasan</div>
          <div class="v">
            {{ data.team.summary.appraised }}
            <span class="text-base font-semibold text-ink-soft">/ {{ data.team.summary.members }}</span>
          </div>
        </div>
        <div class="card">
          <div class="k">Final</div>
          <div class="v">{{ data.team.summary.finalized }}</div>
        </div>
      </div>

      <section class="card !p-2 pb-1">
        <div class="flex flex-wrap items-center gap-3 px-3.5 pb-3 pt-3">
          <div class="min-w-0 flex-1">
            <h2>Validasi realisasi</h2>
            <p class="hint">Cek bukti, lalu setujui atau kembalikan dengan catatan</p>
          </div>
          <button
            type="button"
            class="btn sm pri"
            :disabled="busy || !selected.size"
            @click="approve(pending.filter((p) => selected.has(p.key)))"
          >
            Setujui yang dipilih ({{ selected.size }})
          </button>
        </div>
        <p v-if="!pending.length" class="m-0 px-3.5 pb-4 text-ink-soft">
          Tidak ada realisasi yang menunggu validasi.
        </p>
        <div v-else class="overflow-x-auto">
          <table class="tbl" data-testid="pending-table">
            <thead>
              <tr>
                <th class="w-10">
                  <input
                    type="checkbox"
                    :checked="allSelected"
                    aria-label="Pilih semua"
                    @change="toggleAll"
                  />
                </th>
                <th>Anggota</th>
                <th>Indikator</th>
                <th class="num">Target</th>
                <th class="num">Realisasi</th>
                <th class="num">Capaian</th>
                <th>Bukti</th>
                <th><span class="sr-only">Aksi</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in pending" :key="p.key" :data-testid="`pending-${p.key}`">
                <td>
                  <input
                    type="checkbox"
                    :checked="selected.has(p.key)"
                    :aria-label="`Pilih ${p.employee.name} - ${p.row.indicator.name}`"
                    @change="toggle(p.key)"
                  />
                </td>
                <td>
                  <div class="flex items-center gap-2.5">
                    <Avatar :employee="p.employee" size="s" /><b>{{ p.employee.name }}</b>
                  </div>
                </td>
                <td>
                  {{ p.row.indicator.name }}
                  <div
                    v-if="p.row.result?.note"
                    class="muted max-w-[260px] truncate text-xs"
                    :title="p.row.result.note"
                  >
                    {{ p.row.result.note }}
                  </div>
                </td>
                <td class="num">{{ fmtNum(p.row.indicator.target) }}</td>
                <td class="num">{{ fmtNum(p.row.result?.actual) }}</td>
                <td class="num">{{ fmtScore(p.row.achievement) }}</td>
                <td>
                  <a
                    v-if="p.row.has_evidence"
                    :href="kpi.evidenceUrl(p.row.result!.id)"
                    target="_blank"
                    rel="noopener"
                    >{{ p.row.result?.evidence_name || 'Lihat bukti' }}</a
                  >
                  <span v-else class="muted">tidak ada</span>
                </td>
                <td class="whitespace-nowrap text-right">
                  <button type="button" class="btn sm pri mr-1.5" :disabled="busy" @click="approve([p])">
                    Setujui
                  </button>
                  <button type="button" class="btn sm" :disabled="busy" @click="askReject(p)">
                    Kembalikan
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card !p-2 pb-1">
        <div class="px-3.5 pb-3 pt-3">
          <h2>Status penilaian</h2>
          <p class="hint">{{ data.team.period.name }}</p>
        </div>
        <p v-if="!data.team.rows.length" class="m-0 px-3.5 pb-4 text-ink-soft">Belum ada anggota tim.</p>
        <div v-else class="overflow-x-auto">
          <table class="tbl" data-testid="team-table">
            <thead>
              <tr>
                <th>Anggota</th>
                <th class="num">Skor KPI</th>
                <th>Realisasi KPI</th>
                <th>Self-assessment</th>
                <th>Penilaian atasan</th>
                <th class="num">Skor akhir</th>
                <th><span class="sr-only">Aksi</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in data.team.rows" :key="r.employee.id">
                <td>
                  <div class="flex items-center gap-2.5">
                    <Avatar :employee="r.employee" size="s" />
                    <div>
                      <b>{{ r.employee.name }}</b>
                      <div class="muted text-xs">{{ r.employee.position?.name }}</div>
                    </div>
                  </div>
                </td>
                <td class="num">{{ fmtScore(r.kpi_score) }}</td>
                <td>
                  {{ r.kpi_validated }}/{{ r.kpi_total }} valid<span
                    v-if="r.kpi_pending"
                    class="badge b-warn ml-1.5"
                    >{{ r.kpi_pending }} menunggu</span
                  >
                </td>
                <td>
                  <span class="badge" :class="r.self_done ? 'b-good' : 'b-mute'">{{
                    r.self_done ? 'Selesai' : 'Belum'
                  }}</span>
                </td>
                <td>
                  <span class="badge" :class="APPRAISAL_STATUS[r.status]?.cls">{{
                    r.status === 'draft' ? 'Belum' : APPRAISAL_STATUS[r.status]?.label
                  }}</span>
                </td>
                <td class="num">
                  <template v-if="r.final_score"
                    ><b>{{ fmtScore(r.final_score) }}</b>
                    <span class="badge" :class="gradeClass(r.grade)">{{ r.grade }}</span></template
                  >
                  <template v-else>—</template>
                </td>
                <td class="text-right">
                  <RouterLink
                    :to="`/penilaian/${r.employee.id}`"
                    class="btn sm"
                    :class="{ pri: r.status === 'draft' || r.status === 'self_done' }"
                    >{{ nextAction(r.status) }}</RouterLink
                  >
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
  </PageState>

  <Modal :open="!!rejecting" title="Kembalikan realisasi" @close="rejecting = null">
    <p v-if="rejecting" class="mt-0">
      <b>{{ rejecting.employee.name }}</b> · {{ rejecting.row.indicator.name }}
    </p>
    <label class="field">
      Catatan untuk karyawan
      <textarea
        v-model="reason"
        class="input"
        :class="{ invalid: reasonErr }"
        maxlength="255"
        placeholder="Mis. bukti tidak lengkap atau angka tidak sesuai"
        data-testid="reject-reason"
      />
      <span v-if="reasonErr" class="err">{{ reasonErr }}</span>
    </label>
    <template #footer>
      <button type="button" class="btn" @click="rejecting = null">Batal</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy"
        data-testid="confirm-reject"
        @click="confirmReject"
      >
        Kembalikan
      </button>
    </template>
  </Modal>
</template>
