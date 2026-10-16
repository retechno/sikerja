<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import PageState from '@/components/ui/PageState.vue'
import AppraisalForm from '@/components/appraisal/AppraisalForm.vue'
import Avatar from '@/components/ui/Avatar.vue'
import Modal from '@/components/ui/Modal.vue'
import { ApiError, appraisals, type AppraisalView } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { APPRAISAL_STATUS, fmtDate, fmtScore, gradeClass } from '@/lib/format'
import { useAuthStore } from '@/stores/auth'
import { usePeriodStore } from '@/stores/period'
import { useToastStore } from '@/stores/toast'

// H-07 — Form Penilaian. `self` = karyawan membuka penilaian dirinya sendiri.
const props = defineProps<{ self?: boolean }>()
const route = useRoute()
const auth = useAuthStore()
const period = usePeriodStore()
const toast = useToastStore()

const employeeId = computed(() => (props.self ? (auth.user?.id ?? 0) : Number(route.params.employeeId)))
const view = ref<AppraisalView | null>(null)
const busy = ref(false)
const confirmFinalize = ref(false)

const { loading, error, reload } = useLoader(async () => {
  await period.load()
  if (!period.selectedId) throw new ApiError(404, 'not_found', 'Belum ada periode penilaian')
  view.value = await appraisals.get(period.selectedId, employeeId.value)
  return true
}, [() => period.selectedId, employeeId])

const backTo = computed(() => (props.self ? null : '/penilaian/tim'))
const statusInfo = computed(() => (view.value ? APPRAISAL_STATUS[view.value.status] : null))

async function save(
  p: { scores: Record<number, number>; comment: string; managerComment: string; developmentPlan: string },
  submit: boolean,
) {
  const v = view.value
  if (!v) return
  busy.value = true
  try {
    view.value = v.can_manager
      ? await appraisals.saveManager(v.period.id, v.employee.id, {
          scores: p.scores,
          manager_comment: p.managerComment,
          development_plan: p.developmentPlan,
          submit,
        })
      : await appraisals.saveSelf(v.period.id, v.employee.id, {
          scores: p.scores,
          comment: p.comment,
          submit,
        })
    toast.success(
      submit ? (v.can_manager ? 'Penilaian dikirim.' : 'Self-assessment dikirim.') : 'Draf tersimpan.',
    )
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal menyimpan')
  } finally {
    busy.value = false
  }
}

async function finalize() {
  const v = view.value
  if (!v) return
  busy.value = true
  try {
    view.value = await appraisals.finalize(v.period.id, v.employee.id)
    confirmFinalize.value = false
    toast.success('Penilaian difinalisasi.')
  } catch (e) {
    confirmFinalize.value = false
    toast.error(e instanceof ApiError ? e.message : 'Gagal memfinalisasi')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <PageState :loading="loading" :error="error" @retry="reload">
    <template v-if="view">
      <div class="-mb-2 flex flex-wrap items-center gap-3">
        <p class="hint m-0 flex-1">
          <template v-if="backTo"
            ><RouterLink :to="backTo">Penilaian Tim</RouterLink> / {{ view.employee.name }} ·
          </template>
          {{ view.period.name }}
        </p>
        <span v-if="statusInfo" class="badge" :class="statusInfo.cls">{{ statusInfo.label }}</span>
      </div>

      <div
        v-if="view.period.status !== 'active'"
        class="rounded-xl bg-info-bg px-4 py-3 text-info-fg"
        role="status"
      >
        Periode ini {{ view.period.status === 'closed' ? 'sudah ditutup' : 'belum dibuka' }}; penilaian hanya
        dapat dilihat.
      </div>
      <div
        v-else-if="view.status === 'final'"
        class="rounded-xl bg-good-bg px-4 py-3 text-good-fg"
        role="status"
      >
        Penilaian sudah final{{ view.finalized_at ? ' pada ' + fmtDate(view.finalized_at) : '' }} dan
        terkunci.
      </div>

      <section class="card flex flex-wrap items-center gap-5">
        <div class="flex min-w-0 flex-1 items-center gap-3.5">
          <Avatar :employee="view.employee" size="l" />
          <div class="min-w-0">
            <b class="text-[17px]">{{ view.employee.name }}</b>
            <div class="muted text-xs">
              {{ view.employee.position?.name }} · NIK {{ view.employee.nik }} · masuk
              {{ fmtDate(view.employee.join_date) }}
            </div>
          </div>
        </div>
        <div>
          <div class="muted text-xs">Self-assessment</div>
          <span
            class="badge"
            :class="view.self_submitted_at || view.status !== 'draft' ? 'b-good' : 'b-mute'"
          >
            {{
              view.self_submitted_at
                ? 'Selesai ' + fmtDate(view.self_submitted_at)
                : view.status !== 'draft'
                  ? 'Selesai'
                  : 'Belum'
            }}
          </span>
        </div>
        <div v-if="view.previous" data-testid="previous">
          <div class="muted text-xs">Skor {{ view.previous.period_name }}</div>
          <b class="mr-1.5">{{ fmtScore(view.previous.final_score) }}</b>
          <span class="badge" :class="gradeClass(view.previous.grade)">{{ view.previous.grade }}</span>
        </div>
      </section>

      <AppraisalForm :view="view" :busy="busy" @save="save" @finalize="confirmFinalize = true" />
    </template>
  </PageState>

  <Modal :open="confirmFinalize" title="Finalisasi penilaian" @close="confirmFinalize = false">
    <p class="mt-0">
      Hasil penilaian <b>{{ view?.employee.name }}</b> untuk {{ view?.period.name }} akan dikunci dan skor
      akhir dibekukan. Tindakan ini tercatat di audit log.
    </p>
    <template #footer>
      <button type="button" class="btn" @click="confirmFinalize = false">Batal</button>
      <button type="button" class="btn pri" :disabled="busy" data-testid="confirm-finalize" @click="finalize">
        Finalisasi
      </button>
    </template>
  </Modal>
</template>
