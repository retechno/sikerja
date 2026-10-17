<script setup lang="ts">
import { ref } from 'vue'
import PageState from '@/components/ui/PageState.vue'
import { ApiError, leave, type NotifySetting } from '@/api'
import { useLoader } from '@/lib/useLoader'
import { useToastStore } from '@/stores/toast'

// H-16 (tab "Notifikasi"): saklar per jenis notifikasi. Dikirim di aplikasi dan lewat email.
const toast = useToastStore()
const { data, loading, error, reload } = useLoader(() => leave.notifySettings())
const saved = ref<NotifySetting[] | null>(null)
const busyKey = ref('')

async function toggle(s: NotifySetting) {
  busyKey.value = s.key
  try {
    saved.value = await leave.setNotifySetting(s.key, !s.enabled)
    toast.success(`${s.label}: ${!s.enabled ? 'aktif' : 'nonaktif'}.`)
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Gagal menyimpan')
  } finally {
    busyKey.value = ''
  }
}
const list = () => saved.value ?? data.value ?? []
</script>

<template>
  <PageState :loading="loading && !data" :error="error" @retry="reload">
    <section class="card" data-testid="notif">
      <h2>Notifikasi</h2>
      <p class="hint mb-2">Dikirim di aplikasi (lonceng) dan lewat email</p>
      <div v-for="s in list()" :key="s.key" class="flex items-center gap-3 border-b border-line-soft py-3">
        <div class="min-w-0 flex-1">
          <b>{{ s.label }}</b>
          <div class="muted text-[12.5px]">{{ s.hint }}</div>
        </div>
        <button
          type="button"
          role="switch"
          :aria-checked="s.enabled"
          :aria-label="`${s.label}: ${s.enabled ? 'aktif' : 'nonaktif'}`"
          :disabled="busyKey === s.key"
          class="relative h-7 w-12 flex-none rounded-full transition disabled:opacity-60"
          :class="s.enabled ? 'bg-primary' : 'bg-[#CBD1CA]'"
          :data-testid="`sw-${s.key}`"
          @click="toggle(s)"
        >
          <span
            class="absolute top-0.5 h-6 w-6 rounded-full bg-white shadow transition-all"
            :class="s.enabled ? 'left-[22px]' : 'left-0.5'"
          />
        </button>
      </div>
      <div class="flex items-center gap-3 py-3">
        <div class="min-w-0 flex-1">
          <b>WhatsApp</b>
          <div class="muted text-[12.5px]">tersedia di rilis 3</div>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked="false"
          aria-label="WhatsApp: nonaktif (tersedia di rilis 3)"
          disabled
          class="relative h-7 w-12 flex-none rounded-full bg-[#CBD1CA] opacity-60"
        >
          <span class="absolute left-0.5 top-0.5 h-6 w-6 rounded-full bg-white shadow" />
        </button>
      </div>
    </section>
  </PageState>
</template>
