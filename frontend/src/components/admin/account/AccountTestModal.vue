<template>
  <SharedAccountTestModal
    :show="show"
    :account="account"
    scope="admin"
    @close="emit('close')"
  />
</template>

<script setup lang="ts">
import SharedAccountTestModal from '@/components/account/AccountTestModal.vue'
import type { Account } from '@/types'

defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()
</script>
