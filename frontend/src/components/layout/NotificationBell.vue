<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Icon from '@/components/ui/Icon.vue'
import { notifications, type NotificationItem } from '@/api'

const open = ref(false)
const items = ref<NotificationItem[]>([])
const unread = ref(0)

async function load() {
  try {
    const r = await notifications.list()
    items.value = r.items ?? []
    unread.value = r.unread
  } catch {
    /* lonceng tidak boleh mengganggu halaman */
  }
}

async function readAll() {
  await notifications.readAll()
  await load()
}

function label(n: NotificationItem): string {
  try {
    const d = JSON.parse(n.data) as { message?: string }
    return d.message ?? n.type
  } catch {
    return n.type
  }
}

onMounted(load)
</script>

<template>
  <div class="relative">
    <button
      type="button"
      class="relative grid h-11 w-11 place-items-center rounded-xl border border-line bg-white"
      aria-label="Notifikasi"
      :aria-expanded="open"
      @click="open = !open"
    >
      <Icon name="bell" :size="20" />
      <span v-if="unread > 0" class="absolute right-2.5 top-2.5 h-2 w-2 rounded-full border-2 border-white bg-[#C2410C]" />
    </button>
    <div v-if="open" class="absolute right-0 top-[52px] z-20 w-72 rounded-xl border border-line bg-white shadow-lg">
      <div class="flex items-center justify-between border-b border-line px-4 py-3">
        <b>Notifikasi</b>
        <button v-if="unread > 0" type="button" class="text-xs font-semibold text-primary" @click="readAll">
          Tandai terbaca
        </button>
      </div>
      <p v-if="!items.length" class="m-0 px-4 py-6 text-center text-ink-soft">Belum ada notifikasi.</p>
      <ul v-else class="m-0 max-h-72 list-none overflow-y-auto p-0">
        <li
          v-for="n in items"
          :key="n.id"
          class="border-b border-line-soft px-4 py-3 last:border-0"
          :class="{ 'bg-primary-tint': !n.read_at }"
        >
          {{ label(n) }}
        </li>
      </ul>
    </div>
  </div>
</template>
