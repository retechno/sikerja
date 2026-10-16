<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/ui/Icon.vue'

const props = defineProps<{ open: boolean; title: string; wide?: boolean }>()
const emit = defineEmits<{ close: [] }>()

const panel = ref<HTMLElement | null>(null)

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

watch(
  () => props.open,
  async (o) => {
    if (o) {
      document.addEventListener('keydown', onKey)
      document.body.style.overflow = 'hidden'
      await nextTick()
      panel.value?.querySelector<HTMLElement>('input, select, textarea, button:not([data-close])')?.focus()
    } else {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  },
)
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-end justify-center bg-black/40 p-0 sm:items-center sm:p-4"
      @mousedown.self="emit('close')"
    >
      <div
        ref="panel"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        class="flex max-h-[92vh] w-full flex-col rounded-t-2xl bg-white shadow-xl sm:rounded-2xl"
        :class="wide ? 'sm:max-w-3xl' : 'sm:max-w-lg'"
      >
        <header class="flex items-center gap-3 border-b border-line px-5 py-4">
          <h2 class="m-0 flex-1 text-base font-bold">{{ title }}</h2>
          <button
            data-close
            type="button"
            class="grid h-10 w-10 place-items-center rounded-lg hover:bg-canvas"
            aria-label="Tutup"
            @click="emit('close')"
          >
            <Icon name="x" :size="20" />
          </button>
        </header>
        <div class="overflow-y-auto px-5 py-4"><slot /></div>
        <footer v-if="$slots.footer" class="flex flex-wrap justify-end gap-2.5 border-t border-line px-5 py-3.5">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>
