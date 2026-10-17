<script setup lang="ts">
import { ref, watch } from 'vue'
import { photoUrl } from '@/api'
import { initials } from '@/lib/format'

const props = defineProps<{
  employee?: { id: number; name: string; has_photo?: boolean } | null
  size?: 's' | 'l'
}>()

const failed = ref(false)
watch(
  () => props.employee?.id,
  () => (failed.value = false),
)
</script>

<template>
  <span class="avatar" :class="size" :title="employee?.name">
    <img
      v-if="employee?.has_photo && !failed"
      :src="photoUrl(employee.id)"
      :alt="employee.name"
      class="h-full w-full object-cover"
      @error="failed = true"
    />
    <template v-else>{{ initials(employee?.name) }}</template>
  </span>
</template>
