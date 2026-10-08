<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Message from 'primevue/message'
import { computed, useId } from 'vue'
import { closeGuard } from '@/lib/confirm'
import AppIcon from './AppIcon.vue'
import type { IconName } from './icons'

// Hộp thoại form dùng chung: đầu là dải nền đậm hơn có biểu tượng đặc (brand) và tiêu đề
// lớn, nút đóng tròn; thân là các field (lỗi chung ở trên); chân có gợi ý bên trái, Cancel
// và nút hành động. Enter gửi. Esc hay ✕ khi còn thay đổi chưa lưu thì hỏi "Discard changes?"
const props = withDefaults(
  defineProps<{
    title: string
    icon: IconName
    size?: 's' | 'm' | 'l'
    // bề rộng riêng (hộp thoại báo cáo); không có thì theo size
    width?: string
    // nhãn nút hành động; không có: chân chỉ có Close và thân không là form
    action?: string
    busy?: boolean
    error?: string | null
    dirty?: boolean
    danger?: boolean
    disabled?: boolean
    // thân không padding (nội dung tự chia ngăn)
    flush?: boolean
  }>(),
  { size: 'm' },
)
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ submit: [] }>()

const WIDTH = { s: '26rem', m: '34rem', l: '60rem' } as const
const width = computed(() => props.width ?? `min(${WIDTH[props.size]}, calc(100vw - 2rem))`)
const formId = useId()
const mayClose = closeGuard()

async function requestClose() {
  if (await mayClose(!!props.dirty)) visible.value = false
}
function submit() {
  if (!props.busy && !props.disabled) emit('submit')
}
</script>

<template>
  <Dialog
    :visible="visible"
    modal
    :closable="false"
    :draggable="false"
    :style="{ width }"
    :pt="{
      root: { class: 'form-dialog' },
      header: { class: 'fd-header' },
      content: { class: ['fd-body', { flush }] },
      footer: { class: 'fd-footer' },
    }"
    @update:visible="(v: boolean) => !v && requestClose()"
  >
    <template #header>
      <span class="fd-icon"><AppIcon :name="icon" /></span>
      <h2 :class="['fd-title', { grow: !$slots['header-extra'] }]">{{ title }}</h2>
      <slot name="header-extra" />
      <Button icon="pi pi-times" rounded severity="secondary" class="fd-close" aria-label="Close" @click="requestClose" />
    </template>
    <component :is="action ? 'form' : 'div'" :id="formId" class="fd-content" @submit.prevent="submit">
      <Message v-if="error" severity="error" :closable="false">{{ error }}</Message>
      <slot />
    </component>
    <template #footer>
      <div class="fd-hint"><slot name="hint" /></div>
      <slot name="footer" :close="requestClose">
        <Button :label="action ? 'Cancel' : 'Close'" text severity="secondary" @click="requestClose" />
        <Button
          v-if="action"
          type="submit"
          :form="formId"
          :label="action"
          :loading="busy"
          :disabled="disabled"
          :severity="danger ? 'danger' : undefined"
        />
      </slot>
    </template>
  </Dialog>
</template>

<style>
/* không scoped: Dialog dựng ngoài cây component (portal) */
.form-dialog .fd-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1rem 1.1rem 1rem 1.3rem;
  background: var(--app-soft);
  /* dải nền theo góc bo của hộp thoại, không tràn ra ngoài góc */
  border-top-left-radius: inherit;
  border-top-right-radius: inherit;
}
.form-dialog .fd-icon {
  color: var(--app-brand);
  font-size: 1.8rem;
}
.form-dialog .fd-title {
  flex: 0 1 auto;
  min-width: 0;
  font-family: var(--app-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
}
/* không có gì khác trên đầu: tiêu đề chiếm chỗ trống, nút đóng về phải */
.form-dialog .fd-title.grow {
  flex: 1;
}
.form-dialog .fd-close {
  background: var(--p-content-background);
  color: var(--p-text-color);
  border: 0;
  box-shadow: 0 2px 8px rgb(15 23 42 / 0.15);
}
.form-dialog .fd-body {
  padding-top: 1.1rem;
}
.form-dialog .fd-body.flush {
  padding: 0;
}
/* thân flush: nội dung tự chia ngăn cuộn, cần chiều cao của thân */
.form-dialog .fd-body.flush > .fd-content {
  flex: 1 1 auto;
  min-height: 0;
  gap: 0;
}
.form-dialog .fd-content {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.form-dialog .fd-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.75rem 1.1rem;
  border-top: 1px solid var(--app-line);
}
.form-dialog .fd-hint {
  flex: 1;
  font-size: 0.82rem;
  color: var(--p-text-muted-color);
}
</style>
