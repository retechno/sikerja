<script setup lang="ts">
import { computed, ref } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import Modal from '@/components/ui/Modal.vue'
import Icon from '@/components/ui/Icon.vue'
import LeaveTypesTab from './LeaveTypesTab.vue'
import HolidaysTab from './HolidaysTab.vue'
import NotificationsTab from './NotificationsTab.vue'
import WorkSchedulesTab from './WorkSchedulesTab.vue'
import { ApiError, employees, master, schedules, type Department, type Employee, type Position } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'
import { useAuthStore } from '@/stores/auth'

// H-16 — Pengaturan: departemen & jabatan, aspek perilaku, jenis cuti, hari libur, dan notifikasi.
const toast = useToastStore()
const auth = useAuthStore()

type Tab = 'org' | 'aspects' | 'hours' | 'leave' | 'holidays' | 'notif'
const tab = ref<Tab>('org')
const TABS: { id: Tab; label: string }[] = [
  { id: 'org', label: 'Departemen & jabatan' },
  { id: 'aspects', label: 'Aspek perilaku' },
  { id: 'hours', label: 'Jam kerja' },
  { id: 'leave', label: 'Jenis cuti' },
  { id: 'holidays', label: 'Hari libur' },
  { id: 'notif', label: 'Notifikasi' },
]

const { data, loading, error, reload } = useLoader(async () => {
  const [departments, positions, people, aspects, scheds] = await Promise.all([
    master.departments(),
    master.positions(),
    employees.list({ per_page: 200 }),
    master.aspects(),
    schedules.list(),
  ])
  return { departments, positions, people: people.data, aspects, scheds }
})

const scheduleName = (d: Department) =>
  d.schedule_id
    ? (data.value?.scheds.find((x) => x.id === d.schedule_id)?.name ?? '—')
    : `${data.value?.scheds.find((x) => x.is_default)?.name ?? 'Jadwal default'} (default)`
const headName = (d: Department) => data.value?.people.find((p) => p.id === d.head_id)?.name ?? '—'
const posCount = (d: Department) => data.value?.positions.filter((p) => p.department_id === d.id).length ?? 0
const deptName = (id: number | null) =>
  id ? (data.value?.departments.find((d) => d.id === id)?.name ?? '—') : 'Semua departemen'
const busy = ref(false)

// --- departemen ---
const deptModal = ref(false)
const deptEdit = ref<Department | null>(null)
const deptForm = ref({
  name: '',
  code: '',
  head_id: null as number | null,
  schedule_id: null as number | null,
})
const deptErr = ref<Record<string, string>>({})
const deptMembers = computed<Employee[]>(() =>
  (data.value?.people ?? []).filter((p) => deptEdit.value && p.department_id === deptEdit.value.id),
)

function openDept(d: Department | null) {
  deptEdit.value = d
  deptForm.value = {
    name: d?.name ?? '',
    code: d?.code ?? '',
    head_id: d?.head_id ?? null,
    schedule_id: d?.schedule_id ?? null,
  }
  deptErr.value = {}
  deptModal.value = true
}
async function saveDept() {
  deptErr.value = {}
  if (!deptForm.value.name.trim()) deptErr.value.name = 'Nama wajib diisi'
  if (!deptForm.value.code.trim()) deptErr.value.code = 'Kode wajib diisi'
  if (Object.keys(deptErr.value).length) return
  busy.value = true
  try {
    await master.saveDepartment(deptEdit.value?.id ?? null, {
      ...deptForm.value,
      head_id: deptEdit.value ? deptForm.value.head_id : null,
    })
    toast.success('Departemen disimpan.')
    deptModal.value = false
    await reload()
  } catch (e) {
    if (e instanceof ApiError) deptErr.value = { ...e.fields, form: e.message }
  } finally {
    busy.value = false
  }
}

// --- jabatan ---
const posModal = ref(false)
const posEdit = ref<Position | null>(null)
const posForm = ref({ name: '', department_id: null as number | null })
const posErr = ref<Record<string, string>>({})
function openPos(p: Position | null) {
  posEdit.value = p
  posForm.value = { name: p?.name ?? '', department_id: p?.department_id ?? null }
  posErr.value = {}
  posModal.value = true
}
async function savePos() {
  posErr.value = {}
  if (!posForm.value.name.trim()) {
    posErr.value.name = 'Nama wajib diisi'
    return
  }
  busy.value = true
  try {
    await master.savePosition(posEdit.value?.id ?? null, posForm.value)
    toast.success('Jabatan disimpan.')
    posModal.value = false
    await reload()
  } catch (e) {
    if (e instanceof ApiError) posErr.value = { ...e.fields, form: e.message }
  } finally {
    busy.value = false
  }
}

// --- hapus ---
const removing = ref<{ kind: 'dept' | 'pos'; id: number; name: string } | null>(null)
async function confirmRemove() {
  const r = removing.value
  if (!r) return
  busy.value = true
  try {
    await (r.kind === 'dept' ? master.deleteDepartment(r.id) : master.deletePosition(r.id))
    toast.success(`${r.name} dihapus.`)
    await reload()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal menghapus')
  } finally {
    removing.value = null
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-wrap gap-2" role="tablist" aria-label="Pengaturan">
    <button
      v-for="t in TABS"
      :key="t.id"
      type="button"
      role="tab"
      :aria-selected="tab === t.id"
      class="btn sm"
      :class="{ pri: tab === t.id }"
      @click="tab = t.id"
    >
      {{ t.label }}
    </button>
  </div>

  <WorkSchedulesTab v-if="tab === 'hours'" @changed="reload" />
  <LeaveTypesTab v-else-if="tab === 'leave'" />
  <HolidaysTab v-else-if="tab === 'holidays'" />
  <NotificationsTab v-else-if="tab === 'notif'" />

  <PageState v-else :loading="loading && !data" :error="error" @retry="reload">
    <template v-if="data && tab === 'org'">
      <section class="card !p-2 pb-1">
        <div class="flex items-center gap-3 px-3.5 pb-3 pt-3">
          <div class="flex-1">
            <h2>Departemen</h2>
            <p class="hint">Dipakai untuk template KPI, penilaian, dan cakupan manajer</p>
          </div>
          <button type="button" class="btn sm pri" data-testid="add-dept" @click="openDept(null)">
            <Icon name="plus" :size="15" />Tambah
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="tbl" data-testid="dept-table">
            <thead>
              <tr>
                <th>Departemen</th>
                <th>Kode</th>
                <th>Kepala</th>
                <th>Jadwal kerja</th>
                <th class="num">Jabatan</th>
                <th><span class="sr-only">Aksi</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in data.departments" :key="d.id">
                <td>
                  <b>{{ d.name }}</b>
                </td>
                <td>{{ d.code }}</td>
                <td>{{ headName(d) }}</td>
                <td data-testid="dept-schedule">{{ scheduleName(d) }}</td>
                <td class="num">{{ posCount(d) }}</td>
                <td class="whitespace-nowrap text-right">
                  <button
                    type="button"
                    class="btn sm mr-1.5"
                    :aria-label="`Ubah ${d.name}`"
                    @click="openDept(d)"
                  >
                    Ubah
                  </button>
                  <button
                    type="button"
                    class="btn sm"
                    :aria-label="`Hapus ${d.name}`"
                    @click="removing = { kind: 'dept', id: d.id, name: d.name }"
                  >
                    <Icon name="trash" :size="15" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section class="card !p-2 pb-1">
        <div class="flex items-center gap-3 px-3.5 pb-3 pt-3">
          <div class="flex-1">
            <h2>Jabatan</h2>
            <p class="hint">Jabatan tanpa departemen berlaku lintas departemen (mis. Direktur, Staf HR)</p>
          </div>
          <button type="button" class="btn sm pri" data-testid="add-pos" @click="openPos(null)">
            <Icon name="plus" :size="15" />Tambah
          </button>
        </div>
        <div class="overflow-x-auto">
          <table class="tbl" data-testid="pos-table">
            <thead>
              <tr>
                <th>Jabatan</th>
                <th>Departemen</th>
                <th><span class="sr-only">Aksi</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in data.positions" :key="p.id">
                <td>
                  <b>{{ p.name }}</b>
                </td>
                <td>{{ deptName(p.department_id) }}</td>
                <td class="whitespace-nowrap text-right">
                  <button
                    type="button"
                    class="btn sm mr-1.5"
                    :aria-label="`Ubah ${p.name}`"
                    @click="openPos(p)"
                  >
                    Ubah
                  </button>
                  <button
                    type="button"
                    class="btn sm"
                    :aria-label="`Hapus ${p.name}`"
                    @click="removing = { kind: 'pos', id: p.id, name: p.name }"
                  >
                    <Icon name="trash" :size="15" />
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>

    <section v-else-if="data && tab === 'aspects'" class="card">
      <h2>Aspek perilaku</h2>
      <p class="hint mb-3">
        Dinilai sama untuk semua departemen, skala 1–5. Nilai atasan menjadi skor perilaku (rata-rata × 20).
      </p>
      <ol class="m-0 list-none p-0">
        <li v-for="a in data.aspects" :key="a.id" class="border-b border-line-soft py-3 last:border-b-0">
          <b>{{ a.name }}</b>
          <div class="muted text-[13px]">{{ a.description }}</div>
        </li>
      </ol>
      <p v-if="auth.isHR" class="hint mb-0 mt-3">Perubahan daftar aspek belum tersedia di antarmuka.</p>
    </section>
  </PageState>

  <Modal
    :open="deptModal"
    :title="deptEdit ? 'Ubah departemen' : 'Tambah departemen'"
    @close="deptModal = false"
  >
    <form class="flex flex-col gap-4" novalidate @submit.prevent="saveDept">
      <div v-if="deptErr.form" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">
        {{ deptErr.form }}
      </div>
      <label class="field"
        >Nama<input
          v-model="deptForm.name"
          class="input"
          :class="{ invalid: deptErr.name }"
          data-testid="d-name"
        /><span v-if="deptErr.name" class="err">{{ deptErr.name }}</span></label
      >
      <label class="field"
        >Kode<input
          v-model="deptForm.code"
          class="input"
          :class="{ invalid: deptErr.code }"
          data-testid="d-code"
        /><span v-if="deptErr.code" class="err">{{ deptErr.code }}</span></label
      >
      <label class="field"
        >Jadwal kerja <small>menentukan hari & jam kerja untuk hitungan izin/cuti</small>
        <select v-model="deptForm.schedule_id" class="input" data-testid="d-schedule">
          <option :value="null">Jadwal default perusahaan</option>
          <option v-for="sc in data?.scheds ?? []" :key="sc.id" :value="sc.id">{{ sc.name }}</option>
        </select>
        <span v-if="deptErr.schedule_id" class="err">{{ deptErr.schedule_id }}</span>
      </label>
      <label v-if="deptEdit" class="field"
        >Kepala departemen
        <select v-model="deptForm.head_id" class="input" :class="{ invalid: deptErr.head_id }">
          <option :value="null">— belum ditentukan —</option>
          <option v-for="m in deptMembers" :key="m.id" :value="m.id">{{ m.name }}</option>
        </select>
        <span v-if="deptErr.head_id" class="err">{{ deptErr.head_id }}</span>
      </label>
      <button type="submit" class="sr-only" tabindex="-1">Simpan</button>
    </form>
    <template #footer>
      <button type="button" class="btn" @click="deptModal = false">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="d-save" @click="saveDept">
        Simpan
      </button>
    </template>
  </Modal>

  <Modal :open="posModal" :title="posEdit ? 'Ubah jabatan' : 'Tambah jabatan'" @close="posModal = false">
    <form class="flex flex-col gap-4" novalidate @submit.prevent="savePos">
      <div v-if="posErr.form" class="rounded-xl bg-bad-bg px-4 py-3 text-bad-fg" role="alert">
        {{ posErr.form }}
      </div>
      <label class="field"
        >Nama jabatan<input
          v-model="posForm.name"
          class="input"
          :class="{ invalid: posErr.name }"
          data-testid="p-name"
        /><span v-if="posErr.name" class="err">{{ posErr.name }}</span></label
      >
      <label class="field"
        >Departemen
        <select v-model="posForm.department_id" class="input">
          <option :value="null">Semua departemen</option>
          <option v-for="d in data?.departments ?? []" :key="d.id" :value="d.id">{{ d.name }}</option>
        </select>
      </label>
      <button type="submit" class="sr-only" tabindex="-1">Simpan</button>
    </form>
    <template #footer>
      <button type="button" class="btn" @click="posModal = false">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="p-save" @click="savePos">
        Simpan
      </button>
    </template>
  </Modal>

  <Modal :open="!!removing" title="Hapus data" @close="removing = null">
    <p class="mt-0">
      Hapus <b>{{ removing?.name }}</b
      >? Data yang masih dipakai tidak dapat dihapus.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="removing = null">Batal</button>
      <button
        type="button"
        class="btn danger"
        :disabled="busy"
        data-testid="confirm-remove"
        @click="confirmRemove"
      >
        Hapus
      </button>
    </template>
  </Modal>
</template>
