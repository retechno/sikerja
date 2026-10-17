<script setup lang="ts">
import { ref } from 'vue'
import Modal from '@/components/ui/Modal.vue'
import PageState from '@/components/ui/PageState.vue'
import Icon from '@/components/ui/Icon.vue'
import { ApiError, leave, type LeaveType } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'
import { approvalText, attachmentText, quotaText } from '@/lib/leave'

// H-16 (tab "Jenis cuti"): jenis izin/cuti, kuota, lampiran, setengah hari.
const toast = useToastStore()
const { data, loading, error, reload } = useLoader(() => leave.types())
const { data: policy, reload: reloadPolicy } = useLoader(() => leave.policy())
const policyBusy = ref(false)

async function toggleCarry() {
  if (!policy.value) return
  policyBusy.value = true
  try {
    await leave.setPolicy(!policy.value.carry_over)
    await reloadPolicy()
    toast.success(
      policy.value?.carry_over
        ? 'Sisa saldo akan dibawa ke tahun berikutnya.'
        : 'Sisa saldo hangus di akhir tahun.',
    )
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal menyimpan')
  } finally {
    policyBusy.value = false
  }
}

interface Form {
  name: string
  code: string
  annual_quota: string
  deducts_balance: boolean
  requires_attachment: boolean
  attachment_min_days: string
  allow_half_day: boolean
  min_tenure_months: string
  max_days_per_request: string
  sort: string
  is_active: boolean
}

const open = ref(false)
const editing = ref<LeaveType | null>(null)
const form = ref<Form>(blank())
const errors = ref<Record<string, string>>({})
const busy = ref(false)
const removing = ref<LeaveType | null>(null)

function blank(): Form {
  return {
    name: '',
    code: '',
    annual_quota: '',
    deducts_balance: false,
    requires_attachment: false,
    attachment_min_days: '0',
    allow_half_day: false,
    min_tenure_months: '0',
    max_days_per_request: '',
    sort: '0',
    is_active: true,
  }
}

function openForm(t: LeaveType | null) {
  editing.value = t
  form.value = t
    ? {
        name: t.name,
        code: t.code,
        annual_quota: t.annual_quota ?? '',
        deducts_balance: t.deducts_balance,
        requires_attachment: t.requires_attachment,
        attachment_min_days: String(Number(t.attachment_min_days)),
        allow_half_day: t.allow_half_day,
        min_tenure_months: String(t.min_tenure_months),
        max_days_per_request: t.max_days_per_request ?? '',
        sort: String(t.sort),
        is_active: t.is_active,
      }
    : { ...blank(), sort: String((data.value?.length ?? 0) + 1) }
  errors.value = {}
  open.value = true
}

const numOrNull = (s: string) => (s.trim() === '' ? null : Number(s.replace(',', '.')))

async function save() {
  errors.value = {}
  const f = form.value
  const e: Record<string, string> = {}
  if (!f.name.trim()) e.name = 'Nama wajib diisi'
  if (!f.code.trim()) e.code = 'Kode wajib diisi'
  const quota = numOrNull(f.annual_quota)
  if (quota !== null && (Number.isNaN(quota) || quota < 0)) e.annual_quota = 'Isi angka ≥ 0 atau kosongkan'
  if (f.deducts_balance && quota === null) e.annual_quota = 'Isi kuota per tahun bila saldo dipotong'
  if (Object.keys(e).length) {
    errors.value = e
    return
  }
  busy.value = true
  try {
    await leave.saveType(editing.value?.id ?? null, {
      name: f.name.trim(),
      code: f.code.trim().toUpperCase(),
      annual_quota: quota,
      deducts_balance: f.deducts_balance,
      requires_attachment: f.requires_attachment,
      attachment_min_days: Number(f.attachment_min_days) || 0,
      allow_half_day: f.allow_half_day,
      min_tenure_months: Number(f.min_tenure_months) || 0,
      max_days_per_request: numOrNull(f.max_days_per_request),
      sort: Number(f.sort) || 0,
      is_active: f.is_active,
    })
    toast.success('Jenis izin/cuti disimpan.')
    open.value = false
    await reload()
  } catch (err) {
    if (err instanceof ApiError) errors.value = { ...err.fields, form: err.message }
  } finally {
    busy.value = false
  }
}

async function confirmRemove() {
  const t = removing.value
  if (!t) return
  busy.value = true
  try {
    await leave.deleteType(t.id)
    toast.success(`${t.name} dihapus.`)
    await reload()
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
    <section v-if="policy" class="card mb-4 flex items-center gap-3" data-testid="carry-over">
      <div class="min-w-0 flex-1">
        <b>Bawa sisa saldo ke tahun berikutnya</b>
        <div class="muted text-[12.5px]">
          Nonaktif (bawaan): sisa saldo hangus di akhir tahun. Berlaku untuk saldo yang dibuat setelah diubah.
        </div>
      </div>
      <button
        type="button"
        role="switch"
        :aria-checked="policy.carry_over"
        :aria-label="`Bawa sisa saldo: ${policy.carry_over ? 'aktif' : 'nonaktif'}`"
        :disabled="policyBusy"
        class="relative h-7 w-12 flex-none rounded-full transition disabled:opacity-60"
        :class="policy.carry_over ? 'bg-primary' : 'bg-[#CBD1CA]'"
        data-testid="sw-carry-over"
        @click="toggleCarry"
      >
        <span
          class="absolute top-0.5 h-6 w-6 rounded-full bg-white shadow transition-all"
          :class="policy.carry_over ? 'left-[22px]' : 'left-0.5'"
        />
      </button>
    </section>
    <section v-if="data" class="card !p-2 pb-1" data-testid="leave-types">
      <div class="flex items-center gap-3 px-3.5 pb-3 pt-3">
        <div class="flex-1">
          <h2>Jenis izin &amp; cuti</h2>
          <p class="hint">Kuota per tahun kalender; saldo dibuat otomatis saat pertama dipakai tiap tahun</p>
        </div>
        <button type="button" class="btn sm pri" data-testid="add-type" @click="openForm(null)">
          <Icon name="plus" :size="15" />Tambah jenis
        </button>
      </div>
      <div class="overflow-x-auto">
        <table class="tbl">
          <thead>
            <tr>
              <th>Jenis</th>
              <th>Kode</th>
              <th>Kuota / batas</th>
              <th>Potong saldo</th>
              <th>Wajib lampiran</th>
              <th>Setengah hari</th>
              <th>Persetujuan</th>
              <th><span class="sr-only">Aksi</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in data" :key="t.id" :class="{ 'opacity-60': !t.is_active }">
              <td>
                <b>{{ t.name }}</b
                ><span v-if="!t.is_active" class="badge b-mute ml-2 !py-0">nonaktif</span>
              </td>
              <td>{{ t.code }}</td>
              <td>{{ quotaText(t) }}</td>
              <td :class="t.deducts_balance ? 'font-semibold text-good-fg' : 'text-ink-soft'">
                {{ t.deducts_balance ? 'Ya' : 'Tidak' }}
              </td>
              <td :class="t.requires_attachment ? 'font-semibold text-good-fg' : 'text-ink-soft'">
                {{ attachmentText(t) }}
              </td>
              <td :class="t.allow_half_day ? 'font-semibold text-good-fg' : 'text-ink-soft'">
                {{ t.allow_half_day ? 'Ya' : 'Tidak' }}
              </td>
              <td>{{ approvalText(t) }}</td>
              <td class="whitespace-nowrap text-right">
                <button
                  type="button"
                  class="btn sm mr-1.5"
                  :aria-label="`Ubah ${t.name}`"
                  @click="openForm(t)"
                >
                  Ubah
                </button>
                <button type="button" class="btn sm" :aria-label="`Hapus ${t.name}`" @click="removing = t">
                  <Icon name="trash" :size="15" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="hint px-3.5 pb-3 pt-1">
        Kuota cuti khusus mengikuti kebijakan perusahaan dan peraturan ketenagakerjaan yang berlaku; mohon
        dicek HR sebelum dipakai. Mengubah kuota berlaku untuk saldo tahun berjalan yang belum disesuaikan
        manual.
      </p>
    </section>
  </PageState>

  <Modal
    :open="open"
    :title="editing ? 'Ubah jenis izin/cuti' : 'Tambah jenis izin/cuti'"
    wide
    @close="open = false"
  >
    <form class="grid gap-4 sm:grid-cols-2" novalidate @submit.prevent="save">
      <div v-if="errors.form" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg sm:col-span-2" role="alert">
        {{ errors.form }}
      </div>
      <label class="field"
        >Nama<input
          v-model="form.name"
          class="input"
          :class="{ invalid: errors.name }"
          data-testid="t-name"
        /><span v-if="errors.name" class="err">{{ errors.name }}</span></label
      >
      <label class="field"
        >Kode <small>huruf besar, angka, tanda hubung</small
        ><input
          v-model="form.code"
          class="input"
          :class="{ invalid: errors.code }"
          data-testid="t-code"
        /><span v-if="errors.code" class="err">{{ errors.code }}</span></label
      >
      <label class="field"
        >Kuota per tahun (hari) <small>kosong = tidak dibatasi</small
        ><input
          v-model="form.annual_quota"
          inputmode="decimal"
          class="input"
          :class="{ invalid: errors.annual_quota }"
          data-testid="t-quota"
        /><span v-if="errors.annual_quota" class="err">{{ errors.annual_quota }}</span></label
      >
      <label class="field"
        >Batas hari kerja per pengajuan <small>opsional, mis. cuti menikah 3</small
        ><input
          v-model="form.max_days_per_request"
          inputmode="decimal"
          class="input"
          :class="{ invalid: errors.max_days_per_request }"
        /><span v-if="errors.max_days_per_request" class="err">{{ errors.max_days_per_request }}</span></label
      >
      <label class="field"
        >Masa kerja minimal (bulan)<input
          v-model="form.min_tenure_months"
          inputmode="numeric"
          class="input"
          :class="{ invalid: errors.min_tenure_months }"
        /><span v-if="errors.min_tenure_months" class="err">{{ errors.min_tenure_months }}</span></label
      >
      <label class="field"
        >Lampiran wajib jika lebih dari (hari) <small>0 = selalu wajib bila diaktifkan</small
        ><input
          v-model="form.attachment_min_days"
          inputmode="decimal"
          class="input"
          :disabled="!form.requires_attachment"
      /></label>
      <div class="flex flex-col gap-1 sm:col-span-2">
        <label class="flex min-h-[44px] items-center gap-2.5 font-semibold"
          ><input
            v-model="form.deducts_balance"
            type="checkbox"
            class="h-[18px] w-[18px]"
            data-testid="t-deduct"
          />Potong saldo tahunan saat disetujui (cuti tahunan, izin)</label
        >
        <label class="flex min-h-[44px] items-center gap-2.5 font-semibold"
          ><input v-model="form.requires_attachment" type="checkbox" class="h-[18px] w-[18px]" />Wajib
          lampiran (surat dokter, surat tugas, …)</label
        >
        <label class="flex min-h-[44px] items-center gap-2.5 font-semibold"
          ><input v-model="form.allow_half_day" type="checkbox" class="h-[18px] w-[18px]" />Boleh setengah
          hari (cukup disetujui atasan langsung)</label
        >
        <label class="flex min-h-[44px] items-center gap-2.5 font-semibold"
          ><input v-model="form.is_active" type="checkbox" class="h-[18px] w-[18px]" />Aktif (bisa dipilih
          karyawan)</label
        >
      </div>
      <button type="submit" class="sr-only" tabindex="-1">Simpan</button>
    </form>
    <template #footer>
      <button type="button" class="btn" @click="open = false">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="t-save" @click="save">
        Simpan
      </button>
    </template>
  </Modal>

  <Modal :open="!!removing" title="Hapus jenis" @close="removing = null">
    <p class="mt-0">
      Hapus <b>{{ removing?.name }}</b
      >? Jenis yang sudah pernah dipakai pengajuan tidak dapat dihapus — nonaktifkan saja.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="removing = null">Batal</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy"
        data-testid="t-confirm-remove"
        @click="confirmRemove"
      >
        Hapus
      </button>
    </template>
  </Modal>
</template>
