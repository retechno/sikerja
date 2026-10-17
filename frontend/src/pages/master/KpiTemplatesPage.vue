<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import Icon from '@/components/ui/Icon.vue'
import { ApiError, master, templates, type Department, type Template } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { fmtNum } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { useToastStore } from '@/stores/toast'

// H-13 — Template KPI per departemen. Total bobot wajib 100% (HR mengubah; Direktur hanya melihat).
const auth = useAuthStore()
const toast = useToastStore()
const canEdit = computed(() => auth.isHR)

interface Draft {
  uid: number
  id?: number
  name: string
  unit: string
  target: string
  weight: string
  direction: 'higher' | 'lower'
  source: 'manual' | 'integration'
}

const { data, loading, error, reload } = useLoader(async () => {
  const [depts, tpls] = await Promise.all([master.departments(), templates.list()])
  return { depts, tpls }
})

const deptId = ref<number | null>(null)
const name = ref('')
const active = ref(true)
const rows = ref<Draft[]>([])
const busy = ref(false)
const formError = ref('')
const rowErrors = ref<Record<string, string>>({})
let uid = 0

const template = computed<Template | null>(() => {
  const list = (data.value?.tpls ?? []).filter((t) => t.department_id === deptId.value)
  return list.find((t) => t.is_active) ?? list[0] ?? null
})

function load(t: Template | null) {
  formError.value = ''
  rowErrors.value = {}
  name.value = t?.name ?? ''
  active.value = t?.is_active ?? true
  rows.value = (t?.indicators ?? []).map((i) => ({
    uid: ++uid,
    id: i.id,
    name: i.name,
    unit: i.unit,
    target: String(Number(i.target)),
    weight: String(Number(i.weight)),
    direction: i.direction,
    source: i.source,
  }))
}

watch(
  () => data.value,
  (d) => {
    if (d && (deptId.value === null || !d.depts.some((x) => x.id === deptId.value)))
      deptId.value = d.depts[0]?.id ?? null
    load(template.value)
  },
)
watch(deptId, () => load(template.value))

const total = computed(
  () => Math.round(rows.value.reduce((a, r) => a + (Number(r.weight) || 0), 0) * 1000) / 1000,
)
const totalOk = computed(() => Math.abs(total.value - 100) <= 0.001)
const dept = computed<Department | undefined>(() => data.value?.depts.find((d) => d.id === deptId.value))

function addRow() {
  rows.value.push({
    uid: ++uid,
    name: '',
    unit: '',
    target: '',
    weight: '',
    direction: 'higher',
    source: 'manual',
  })
}
function removeRow(i: number) {
  rows.value.splice(i, 1)
}

function validate(): boolean {
  const e: Record<string, string> = {}
  rows.value.forEach((r, i) => {
    if (!r.name.trim()) e[`${i}.name`] = 'Wajib diisi'
    if (!(Number(r.target) > 0)) e[`${i}.target`] = 'Harus > 0'
    if (!(Number(r.weight) > 0)) e[`${i}.weight`] = 'Harus > 0'
  })
  if (!name.value.trim()) e.name = 'Nama template wajib diisi'
  if (!rows.value.length) formError.value = 'Tambahkan minimal satu indikator'
  else if (!totalOk.value) formError.value = `Total bobot harus 100% (saat ini ${fmtNum(total.value)}%)`
  else formError.value = ''
  rowErrors.value = e
  return Object.keys(e).length === 0 && !formError.value
}

async function save() {
  if (!deptId.value || !validate()) return
  busy.value = true
  try {
    let t = template.value
    if (!t) t = await templates.create(deptId.value, name.value.trim())
    await templates.save(t.id, {
      name: name.value.trim(),
      is_active: active.value,
      indicators: rows.value.map((r) => ({
        id: r.id,
        name: r.name.trim(),
        unit: r.unit.trim(),
        target: Number(r.target),
        weight: Number(r.weight),
        direction: r.direction,
        source: r.source,
      })),
    })
    toast.success('Template KPI disimpan.')
    await reload()
  } catch (e) {
    formError.value = e instanceof ApiError ? (e.fields.indicators ?? e.message) : 'Gagal menyimpan'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PageState :loading="loading && !data" :error="error" @retry="reload">
    <template v-if="data">
      <div class="flex flex-wrap gap-2" role="tablist" aria-label="Departemen">
        <button
          v-for="d in data.depts"
          :key="d.id"
          type="button"
          role="tab"
          :aria-selected="deptId === d.id"
          class="btn sm"
          :class="{ pri: deptId === d.id }"
          @click="deptId = d.id"
        >
          {{ d.name }}
        </button>
      </div>

      <section class="card" data-testid="template-card">
        <div class="mb-4 flex flex-wrap items-start gap-4">
          <div class="min-w-0 flex-1">
            <h2>KPI {{ dept?.name }}</h2>
            <p class="hint">
              {{ rows.length }} indikator<template v-if="!template">
                · belum ada template untuk departemen ini</template
              >
            </p>
          </div>
          <div class="text-right">
            <div class="k">Total bobot</div>
            <span class="badge !text-sm" :class="totalOk ? 'b-good' : 'b-bad'" data-testid="total-weight"
              >{{ fmtNum(total) }}%</span
            >
          </div>
        </div>

        <div v-if="!canEdit" class="mb-4 rounded-xl bg-info-bg px-4 py-3 text-info-fg">
          Hanya HR yang dapat mengubah template. Anda dapat melihatnya.
        </div>
        <div
          v-if="formError"
          class="mb-4 rounded-xl bg-bad-bg px-4 py-3 text-bad-fg"
          role="alert"
          data-testid="form-error"
        >
          {{ formError }}
        </div>

        <div class="mb-4 grid gap-4 sm:grid-cols-[1fr_auto]">
          <label class="field"
            >Nama template
            <input v-model="name" class="input" :class="{ invalid: rowErrors.name }" :disabled="!canEdit" />
            <span v-if="rowErrors.name" class="err">{{ rowErrors.name }}</span>
          </label>
          <label class="flex min-h-[44px] items-end gap-2 pb-2.5 font-semibold">
            <input v-model="active" type="checkbox" class="h-[18px] w-[18px]" :disabled="!canEdit" />Template
            aktif
          </label>
        </div>

        <div class="overflow-x-auto">
          <table class="tbl" data-testid="indicator-table">
            <thead>
              <tr>
                <th class="min-w-[220px]">Indikator</th>
                <th>Satuan</th>
                <th class="num">Target / bulan</th>
                <th>Arah</th>
                <th class="num">Bobot (%)</th>
                <th>Sumber data</th>
                <th v-if="canEdit"><span class="sr-only">Hapus</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(r, i) in rows" :key="r.uid" :data-testid="`ind-${i}`">
                <td>
                  <input
                    v-model="r.name"
                    class="input"
                    :class="{ invalid: rowErrors[`${i}.name`] }"
                    :disabled="!canEdit"
                    :aria-label="`Nama indikator ${i + 1}`"
                  />
                </td>
                <td>
                  <input
                    v-model="r.unit"
                    class="input !w-28"
                    :disabled="!canEdit"
                    :aria-label="`Satuan ${i + 1}`"
                  />
                </td>
                <td>
                  <input
                    v-model="r.target"
                    type="number"
                    step="any"
                    min="0"
                    class="input !w-28 text-right"
                    :class="{ invalid: rowErrors[`${i}.target`] }"
                    :disabled="!canEdit"
                    :aria-label="`Target ${i + 1}`"
                  />
                </td>
                <td>
                  <select
                    v-model="r.direction"
                    class="input"
                    :disabled="!canEdit"
                    :aria-label="`Arah ${i + 1}`"
                  >
                    <option value="higher">Lebih tinggi lebih baik</option>
                    <option value="lower">Lebih rendah lebih baik</option>
                  </select>
                </td>
                <td>
                  <input
                    v-model="r.weight"
                    type="number"
                    step="any"
                    min="0"
                    class="input !w-24 text-right"
                    :class="{ invalid: rowErrors[`${i}.weight`] }"
                    :disabled="!canEdit"
                    :aria-label="`Bobot ${i + 1}`"
                    data-weight
                  />
                </td>
                <td>
                  <select
                    v-model="r.source"
                    class="input"
                    :disabled="!canEdit"
                    :aria-label="`Sumber ${i + 1}`"
                  >
                    <option value="manual">Manual</option>
                    <option value="integration">Integrasi</option>
                  </select>
                </td>
                <td v-if="canEdit">
                  <button
                    type="button"
                    class="btn sm"
                    :aria-label="`Hapus indikator ${i + 1}`"
                    @click="removeRow(i)"
                  >
                    <Icon name="trash" :size="15" />
                  </button>
                </td>
              </tr>
            </tbody>
            <tfoot>
              <tr>
                <td colspan="4">Total bobot</td>
                <td class="num">{{ fmtNum(total) }}%</td>
                <td colspan="2"></td>
              </tr>
            </tfoot>
          </table>
        </div>

        <div v-if="canEdit" class="mt-4 flex flex-wrap items-center gap-3">
          <button type="button" class="btn sm" data-testid="add-indicator" @click="addRow">
            <Icon name="plus" :size="15" />Tambah indikator
          </button>
          <span class="flex-1" />
          <button type="button" class="btn" :disabled="busy" @click="load(template)">
            Batalkan perubahan
          </button>
          <button
            type="button"
            class="btn pri"
            :disabled="busy || !totalOk"
            data-testid="save-template"
            @click="save"
          >
            {{ busy ? 'Menyimpan…' : 'Simpan template' }}
          </button>
        </div>
      </section>

      <div class="rounded-xl bg-info-bg px-4 py-3 text-info-fg">
        Template tidak bisa disimpan jika total bobot bukan 100%. Sumber "Integrasi" disiapkan untuk rilis 3
        (data otomatis dari helpdesk, CRM, atau Jira); saat ini semua realisasi diinput manual.
      </div>
    </template>
  </PageState>
</template>
