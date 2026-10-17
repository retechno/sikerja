<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import Modal from '@/components/ui/Modal.vue'
import Icon from '@/components/ui/Icon.vue'
import LeaveTimeline from '@/components/leave/LeaveTimeline.vue'
import { ApiError, leave, type LeaveRequest } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'
import { fmtDate, fmtTime } from '@/lib/format'
import { fmtDays, fmtShortDay, fmtWeekdayDate, LEAVE_STATUS, leaveRangeText, leaveTitle } from '@/lib/leave'

// H-09 — Riwayat Pengajuan: daftar + status + linimasa persetujuan, batalkan.
const route = useRoute()
const router = useRouter()
const toast = useToastStore()

const thisYear = new Date().getFullYear()
const year = ref(thisYear)
const status = ref('')
const years = [thisYear, thisYear - 1, thisYear - 2]

const { data, loading, error, reload } = useLoader(
  () => leave.list({ year: year.value, status: status.value || undefined, per_page: 100 }),
  [year, status],
)
const rows = computed(() => data.value?.data ?? [])

// pengajuan terpilih (?id=) → detail lengkap dari server
const selectedId = computed(() => Number(route.query.id) || null)
const detail = ref<LeaveRequest | null>(null)
const detailError = ref('')
let seq = 0
async function loadDetail() {
  const id = selectedId.value
  detail.value = null
  detailError.value = ''
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

// buka otomatis pengajuan terbaru bila belum ada yang dipilih (desktop)
watch(rows, (r) => {
  if (!selectedId.value && r.length && window.matchMedia?.('(min-width: 1024px)').matches) {
    void router.replace({ query: { ...route.query, id: String(r[0]!.id) } })
  }
})

const select = (id: number) => router.replace({ query: { ...route.query, id: String(id) } })

// ---- batalkan ----
const confirming = ref(false)
const busy = ref(false)
async function cancel() {
  if (!detail.value) return
  busy.value = true
  try {
    detail.value = await leave.cancel(detail.value.id)
    toast.success(`Pengajuan ${detail.value.number ?? ''} dibatalkan.`)
    confirming.value = false
    await reload()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal membatalkan')
    confirming.value = false
    await loadDetail()
  } finally {
    busy.value = false
  }
}

const dayCount = (l: LeaveRequest) => fmtDays(l.days)
</script>

<template>
  <div class="flex flex-wrap items-center gap-3">
    <div class="min-w-0 flex-1">
      <h2 class="m-0 text-xl font-bold">Riwayat pengajuan</h2>
      <p class="hint" data-testid="count">{{ data?.meta.total ?? 0 }} pengajuan di {{ year }}</p>
    </div>
    <RouterLink to="/izin-cuti/ajukan" class="btn pri"><Icon name="plus" :size="16" />Ajukan baru</RouterLink>
  </div>

  <div class="flex flex-wrap items-start gap-6">
    <section class="card min-w-0 flex-[2_1_520px] !p-2 pb-1">
      <div class="flex flex-wrap items-center gap-4 px-3.5 pb-1 pt-3">
        <label class="flex items-center gap-2 text-[13px] font-semibold"
          >Tahun
          <select v-model.number="year" class="input !min-h-[40px] !w-auto" aria-label="Tahun">
            <option v-for="y in years" :key="y" :value="y">{{ y }}</option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-[13px] font-semibold"
          >Status
          <select v-model="status" class="input !min-h-[40px] !w-auto" aria-label="Status">
            <option value="">Semua status</option>
            <option value="pending">Menunggu</option>
            <option value="approved">Disetujui</option>
            <option value="rejected">Ditolak</option>
            <option value="cancelled">Dibatalkan</option>
          </select>
        </label>
      </div>
      <PageState
        :loading="loading && !data"
        :error="error"
        :empty="!rows.length"
        empty-text="Belum ada pengajuan pada filter ini."
        @retry="reload"
      >
        <div class="overflow-x-auto">
          <table class="tbl" data-testid="history">
            <thead>
              <tr>
                <th>Nomor</th>
                <th>Jenis</th>
                <th>Tanggal</th>
                <th class="num">Hari</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="l in rows"
                :key="l.id"
                class="cursor-pointer hover:bg-canvas"
                :class="{ sel: l.id === selectedId }"
                tabindex="0"
                :aria-selected="l.id === selectedId"
                @click="select(l.id)"
                @keydown.enter="select(l.id)"
              >
                <td class="whitespace-nowrap font-mono text-xs text-ink-soft">{{ l.number }}</td>
                <td>
                  <b>{{ leaveTitle(l) }}</b>
                </td>
                <td class="whitespace-nowrap">{{ leaveRangeText(l) }}</td>
                <td class="num">{{ dayCount(l) }}</td>
                <td>
                  <span class="badge" :class="LEAVE_STATUS[l.status].cls">{{
                    LEAVE_STATUS[l.status].label
                  }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </PageState>
    </section>

    <section class="card min-w-0 flex-[1_1_340px]" data-testid="detail" aria-live="polite">
      <p v-if="detailError" class="m-0 font-semibold text-bad-fg" role="alert">{{ detailError }}</p>
      <p v-else-if="!detail" class="muted m-0">Pilih pengajuan untuk melihat rincian dan linimasanya.</p>
      <template v-else>
        <div class="mb-4 flex items-start gap-3">
          <div class="min-w-0 flex-1">
            <p class="m-0 font-mono text-xs text-ink-soft">{{ detail.number }}</p>
            <h2>{{ leaveTitle(detail) }} · {{ fmtDays(detail.days) }} hari kerja</h2>
          </div>
          <span class="badge" :class="LEAVE_STATUS[detail.status].cls">{{
            LEAVE_STATUS[detail.status].label
          }}</span>
        </div>

        <dl class="m-0 mb-5 grid grid-cols-[110px_1fr] gap-x-3 gap-y-2.5 text-[13.5px]">
          <dt class="muted">Tanggal</dt>
          <dd class="m-0 font-semibold">
            <template v-if="detail.start_date.slice(0, 10) === detail.end_date.slice(0, 10)">{{
              fmtWeekdayDate(detail.start_date.slice(0, 10))
            }}</template>
            <template v-else
              >{{ fmtShortDay(detail.start_date.slice(0, 10)) }} –
              {{ fmtShortDay(detail.end_date.slice(0, 10)) }} {{ detail.end_date.slice(0, 4) }}</template
            >
            <template v-if="detail.half_day && detail.half_session"> ({{ detail.half_session }})</template>
          </dd>
          <template v-if="detail.skipped?.length">
            <dt class="muted">Tidak dihitung</dt>
            <dd class="m-0 font-semibold">
              {{
                detail.skipped
                  .map(
                    (d) =>
                      fmtShortDay(d.date).replace(/ [A-Z][a-z]{2}$/, '') +
                      (d.holiday ? ` (${d.holiday})` : ''),
                  )
                  .join(', ')
              }}
            </dd>
          </template>
          <dt class="muted">Alasan</dt>
          <dd class="m-0 font-semibold">{{ detail.reason }}</dd>
          <template v-if="detail.substitute">
            <dt class="muted">Pengganti</dt>
            <dd class="m-0 font-semibold">{{ detail.substitute.name }}</dd>
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

        <h3 class="mb-3 mt-0 text-sm font-bold">Linimasa</h3>
        <LeaveTimeline v-if="detail.timeline" :steps="detail.timeline" />

        <template v-if="detail.can_cancel">
          <button
            type="button"
            class="btn danger mt-5 w-full"
            data-testid="cancel"
            @click="confirming = true"
          >
            Batalkan pengajuan
          </button>
          <p v-if="detail.cancel_until" class="hint mt-2 text-center">
            Bisa dibatalkan sampai {{ fmtDate(detail.cancel_until) }}.
            <template v-if="detail.deducted">Saldo yang sudah dipotong akan dikembalikan.</template>
          </p>
        </template>
      </template>
    </section>
  </div>

  <Modal :open="confirming" title="Batalkan pengajuan" @close="confirming = false">
    <p class="mt-0">
      Batalkan pengajuan <b>{{ detail?.number }}</b
      >? Penyetuju akan diberi tahu<template v-if="detail?.deducted"> dan saldo cuti dikembalikan</template>.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="confirming = false">Tidak</button>
      <button type="button" class="btn danger" :disabled="busy" data-testid="confirm-cancel" @click="cancel">
        Ya, batalkan
      </button>
    </template>
  </Modal>
</template>
