<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Textarea from 'primevue/textarea'
import { ref, watch } from 'vue'
import { notify } from '@/lib/notify'
import { useRetireAsset } from '../api'

// Hộp thoại retire dùng chung cho danh sách và trang tài sản
const props = defineProps<{ asset: { id: string; tag: string; name: string; version: number } | null }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ retired: [] }>()

const reason = ref('')
const retire = useRetireAsset()

watch(visible, (open) => {
  if (open) reason.value = ''
})

async function submit() {
  const a = props.asset
  if (!a) return
  try {
    await retire.mutateAsync({ id: a.id, reason: reason.value, version: a.version })
    visible.value = false
    notify.success(`${a.tag} retired.`)
    emit('retired')
  } catch {
    // lỗi đã hiện bằng toast mặc định của mutation
  }
}
</script>

<template>
  <Dialog v-model:visible="visible" modal :header="asset ? `Retire ${asset.tag}?` : 'Retire asset'" :style="{ width: '30rem' }">
    <form class="form" @submit.prevent="submit">
      <p>{{ asset?.name }} leaves the list unless “Include retired” is on, and can't be edited until restored.</p>
      <div class="field">
        <label for="retire-reason">Reason (optional)</label>
        <Textarea id="retire-reason" v-model="reason" rows="3" />
      </div>
      <div class="actions">
        <Button type="submit" label="Retire" severity="danger" :loading="retire.isPending.value" />
        <Button label="Cancel" severity="secondary" text @click="visible = false" />
      </div>
    </form>
  </Dialog>
</template>
