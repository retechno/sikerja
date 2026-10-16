<script setup lang="ts">
// Skala 1–5 (radiogroup) seperti mockup H-07.
defineProps<{ modelValue: number | null; label: string; disabled?: boolean }>()
defineEmits<{ 'update:modelValue': [value: number] }>()
</script>

<template>
  <div class="flex gap-1.5" role="radiogroup" :aria-label="label">
    <button
      v-for="n in 5"
      :key="n"
      type="button"
      role="radio"
      :aria-checked="modelValue === n"
      :disabled="disabled"
      class="h-11 w-11 rounded-[10px] border font-bold transition disabled:cursor-not-allowed"
      :class="
        modelValue === n
          ? 'border-primary bg-primary text-white'
          : 'border-line-input bg-white text-mute-fg hover:bg-canvas disabled:hover:bg-white'
      "
      @click="$emit('update:modelValue', n)"
    >
      {{ n }}
    </button>
  </div>
</template>
