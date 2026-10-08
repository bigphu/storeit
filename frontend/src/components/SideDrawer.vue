<script setup lang="ts">
import Drawer from 'primevue/drawer'

// Ngăn kéo bên cạnh dùng chung (sửa nhanh bên phải, điều hướng trên màn hẹp bên trái): một chỗ
// cho vị trí, bề rộng, lớp CSS các phần, và sự kiện closed khi hiệu ứng trượt ra đã xong (cha
// giữ nội dung đến lúc đó rồi mới bỏ, xem lib/quickDrawer.ts). Cha có thể chặn việc đóng: nghe
// update:visible và chỉ đặt lại visible khi đồng ý
withDefaults(
  defineProps<{
    visible: boolean
    position?: 'left' | 'right'
    width?: string
    header?: string
    modal?: boolean
    dismissable?: boolean
    closeOnEscape?: boolean
    showCloseIcon?: boolean
    rootClass?: string
    headerClass?: string
    contentClass?: string
    footerClass?: string
  }>(),
  { position: 'right', width: 'min(26rem, 100vw)', modal: true, dismissable: true, closeOnEscape: true, showCloseIcon: true },
)
const emit = defineEmits<{ 'update:visible': [value: boolean]; closed: [] }>()
</script>

<template>
  <Drawer
    :visible="visible"
    :position="position"
    :header="header"
    :modal="modal"
    :dismissable="dismissable"
    :close-on-escape="closeOnEscape"
    :show-close-icon="showCloseIcon"
    :pt="{
      root: { class: ['side-drawer', rootClass], style: `width: ${width}` },
      header: { class: headerClass },
      content: { class: contentClass },
      footer: { class: footerClass },
    }"
    @update:visible="(v: boolean) => emit('update:visible', v)"
    @after-hide="emit('closed')"
  >
    <template v-if="$slots.header" #header><slot name="header" /></template>
    <slot />
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </Drawer>
</template>
