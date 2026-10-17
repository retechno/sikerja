<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import Avatar from '@/components/ui/Avatar.vue'
import LeaveTimeline from '@/components/leave/LeaveTimeline.vue'
import { ApiError, leave, type LeaveRequest } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'
import { useAuthStore } from '@/stores/auth'
import { useLeaveStore } from '@/stores/leave'
import { fmtDate, fmtTime } from '@/lib/format'
import { fmtDays, fmtWeekdayDate, LEAVE_STATUS, leaveRangeText, leaveTitle, relativeDay } from '@/lib/leave'

// H-10 — Persetujuan: antrean pengajuan, detail, setujui/tolak dengan catatan.
const route = useRoute()
const router = useRouter()
const toast = useToastStore()
const auth = useAuthStore()
const leaveStore = useLeaveStore()

const tab = ref<'pending' | 'history'>('pending')
const { data, loading, error, reload } = useLoader(() => leave.approvals(tab.value), [tab])
const rows = computed(() => data.value ?? [])

const subtitle = computed(() => {
  const parts: string[] = []
  if (auth.isManager) parts.push(`tahap 1 untuk tim ${auth.user?.department?.name ?? ''}`.trim())
  if (auth.isDirector) parts.push('tahap 1 untuk manajer')
  if (auth.isHR) parts.push('tahap akhir untuk seluruh perusahaan')
  return parts.length ? `Anda adalah penyetuju ${parts.join(' dan ')}` : ''
})

const selectedId = ref<number | null>(Number(route.query.id) || null)
watch(rows, (r) => {
  if (!r.length) selectedId.value = null
  else if (!r.some((x) => x.id === selectedId.value) && window.matchMedia?.('(min-width: 1024px)').matches)
    selectedId.value = r[0]!.id
})
watch(tab, () => {
  selectedId.value = null
})

const detail = ref<LeaveRequest | null>(null)
const detailError = ref('')
const note = ref('')
const noteErr = ref('')
let seq = 0
async function loadDetail() {
  detail.value = null
  detailError.value = ''
  note.value = ''
  noteErr.value = ''
  const id = selectedId.value
  if (!id) return
  const my = ++seq
  try {
    const d = await leave.get(id)
    if (my === seq) detail.value = d
  } catch (e) {
    if (my === seq) detailError.value = e instanceof ApiError ? e.message : 'Gagal memuat detail'
  }
}
watch(selectedId, loadDetail, { immediate: true })
function select(id: number) {
  selectedId.value = id
  void router.replace({ query: { ...route.query, id: String(id) } })
}

const busy = ref(false)
async function decide(approve: boolean) {
  if (!detail.value) return
  noteErr.value = ''
  if (!approve && !note.value.trim()) {
    noteErr.value = 'Catatan wajib diisi jika menolak.'
    return
  }
  busy.value = true
  const id = detail.value.id
  const name = detail.value.employee?.name ?? ''
  try {
    if (approve) await leave.approve(id, note.value.trim())
    else await leave.reject(id, note.value.trim())
    toast.success(approve ? `Pengajuan ${name} disetujui.` : `Pengajuan ${name} ditolak.`)
    selectedId.value = null
    await reload()
    void leaveStore.refresh()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : 'Gagal memproses pengajuan'
    if (e instanceof ApiError && e.fields.note) noteErr.value = e.fields.note
    else toast.error(msg)
    if (e instanceof ApiError && e.status === 409) {
      await reload()
      await loadDetail()
    }
  } finally {
    busy.value = false
  }
}

const approveLabel = computed(() => (detail.value?.is_last_stage ? 'Setujui' : 'Setujui & teruskan ke HR'))
const conflictText = computed(() => {
  const c = detail.value?.conflicts ?? []
  return c.map((x) => `${x.employee_name} (${x.type_name}, ${x.range})`).join('; ')
})
</script>

<template>
  <div>
    <h2 class="m-0 text-xl font-bold">Persetujuan izin &amp; cuti</h2>
    <p v-if="subtitle" class="hint">{{ subtitle }}</p>
  </div>

  <nav class="flex gap-2" role="tablist" aria-label="Antrean">
    <button
      type="button"
      role="tab"
      class="btn sm"
      :class="{ pri: tab === 'pending' }"
      :aria-selected="tab === 'pending'"
      data-testid="tab-pending"
      @click="tab = 'pending'"
    >
      Menunggu
      <span v-if="tab === 'pending' && rows.length" class="badge b-warn">{{ rows.length }}</span>
    </button>
    <button
      type="button"
      role="tab"
      class="btn sm"
      :class="{ pri: tab === 'history' }"
      :aria-selected="tab === 'history'"
      data-testid="tab-history"
      @click="tab = 'history'"
    >
      Sudah diproses
    </button>
  </nav>

  <PageState
    :loading="loading && !data"
    :error="error"
    :empty="!rows.length"
    :empty-text="
      tab === 'pending'
        ? 'Tidak ada pengajuan yang menunggu keputusan Anda.'
        : 'Belum ada pengajuan yang Anda proses.'
    "
    @retry="reload"
  >
    <div class="flex flex-wrap items-start gap-6">
      <section
        class="flex min-w-0 flex-[1_1_320px] flex-col gap-2.5"
        data-testid="queue"
        aria-label="Antrean"
      >
        <button
          v-for="l in rows"
          :key="l.id"
          type="button"
          class="flex min-h-[60px] items-center gap-3 rounded-2xl border bg-white px-3.5 py-3 text-left transition hover:bg-canvas"
          :class="l.id === selectedId ? 'border-primary ring-1 ring-primary' : 'border-line'"
          :aria-current="l.id === selectedId"
          :data-testid="`q-${l.id}`"
          @click="select(l.id)"
        >
          <Avatar :employee="l.employee" />
          <span class="min-w-0 flex-1">
            <b class="block truncate">{{ l.employee?.name }}</b>
            <span class="muted block truncate text-[12.5px]"
              >{{ leaveTitle(l) }} · {{ leaveRangeText(l) }} · {{ fmtDays(l.days) }} hari</span
            >
          </span>
          <span v-if="tab === 'pending'" class="muted text-xs">{{ relativeDay(l.created_at) }}</span>
          <span v-else class="badge" :class="LEAVE_STATUS[l.status].cls">{{
            LEAVE_STATUS[l.status].label
          }}</span>
        </button>
      </section>

      <section
        class="card flex min-w-0 flex-[2_1_480px] flex-col gap-[18px]"
        data-testid="detail"
        aria-live="polite"
      >
        <p v-if="detailError" class="m-0 font-semibold text-bad-fg" role="alert">{{ detailError }}</p>
        <p v-else-if="!detail" class="muted m-0">Pilih pengajuan di sebelah kiri untuk melihat rincian.</p>
        <template v-else>
          <div class="flex flex-wrap items-center gap-3">
            <Avatar :employee="detail.employee" size="l" />
            <div class="min-w-0 flex-1">
              <b class="block text-[17px]">{{ detail.employee?.name }}</b>
              <span class="muted text-[13px]"
                >{{ detail.employee?.position?.name ?? detail.employee?.department?.name }} ·
                {{ detail.number }}</span
              >
            </div>
            <span class="badge" :class="LEAVE_STATUS[detail.status].cls">{{
              LEAVE_STATUS[detail.status].label
            }}</span>
          </div>

          <dl class="m-0 grid grid-cols-[130px_1fr] gap-x-3 gap-y-2.5 text-[13.5px]">
            <dt class="muted">Jenis</dt>
            <dd class="m-0 font-semibold">{{ leaveTitle(detail) }}</dd>
            <dt class="muted">Tanggal</dt>
            <dd class="m-0 font-semibold">
              {{ leaveRangeText(detail) }} ({{ fmtDays(detail.days) }} hari kerja)
              <span
                v-if="detail.start_date.slice(0, 10) === detail.end_date.slice(0, 10)"
                class="muted font-normal"
                >· {{ fmtWeekdayDate(detail.start_date.slice(0, 10)).split(',')[0] }}</span
              >
            </dd>
            <dt class="muted">Alasan</dt>
            <dd class="m-0 font-semibold">{{ detail.reason }}</dd>
            <template v-if="detail.substitute">
              <dt class="muted">Pengganti</dt>
              <dd class="m-0 font-semibold">{{ detail.substitute.name }}</dd>
            </template>
            <template v-if="detail.balance">
              <dt class="muted">Saldo {{ detail.leave_type?.name?.toLowerCase() }}</dt>
              <dd class="m-0 font-semibold" data-testid="balance">
                {{ fmtDays(detail.balance.available) }} hari → sisa
                {{ fmtDays(detail.balance.after) }} setelah disetujui
              </dd>
            </template>
            <template v-if="detail.has_attachment">
              <dt class="muted">Lampiran</dt>
              <dd class="m-0 font-semibold">
                <a :href="leave.attachmentUrl(detail.id)" target="_blank" rel="noopener">{{
                  detail.attachment_name || 'Lihat lampiran'
                }}</a>
              </dd>
            </template>
            <dt class="muted">Diajukan</dt>
            <dd class="m-0 font-semibold">
              {{ fmtDate(detail.created_at) }}, {{ fmtTime(detail.created_at) }}
            </dd>
          </dl>

          <div
            v-if="detail.status === 'pending_manager' || detail.status === 'pending_hr'"
            class="flex items-start gap-2.5 rounded-xl px-3.5 py-3 text-[13.5px]"
            :class="detail.conflicts?.length ? 'bg-warn-bg text-warn-fg' : 'bg-good-bg text-good-fg'"
            data-testid="conflict"
          >
            <b>{{ detail.conflicts?.length ? 'Ada yang bentrok.' : 'Tidak bentrok.' }}</b>
            <span v-if="detail.conflicts?.length"
              >Rekan satu departemen yang juga izin/cuti: {{ conflictText }}.</span
            >
            <span v-else>Tidak ada rekan satu departemen yang izin/cuti pada tanggal ini.</span>
          </div>

          <template v-if="detail.can_act">
            <label class="field"
              >Catatan untuk karyawan <small>wajib jika menolak</small>
              <textarea
                v-model="note"
                class="input"
                :class="{ invalid: noteErr }"
                maxlength="500"
                placeholder="Contoh: pastikan serah terima tiket aktif sebelum cuti."
                data-testid="note"
              />
              <span v-if="noteErr" class="err" role="alert">{{ noteErr }}</span>
            </label>
            <div class="flex flex-wrap justify-end gap-2.5">
              <button
                type="button"
                class="btn danger"
                :disabled="busy"
                data-testid="reject"
                @click="decide(false)"
              >
                Tolak
              </button>
              <button
                type="button"
                class="btn pri"
                :disabled="busy"
                data-testid="approve"
                @click="decide(true)"
              >
                {{ approveLabel }}
              </button>
            </div>
          </template>

          <div v-if="detail.timeline">
            <h3 class="mb-3 mt-0 text-sm font-bold">Linimasa</h3>
            <LeaveTimeline :steps="detail.timeline" />
          </div>
        </template>
      </section>
    </div>
  </PageState>
</template>
