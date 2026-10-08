<script setup lang="ts">
import ContextMenu from 'primevue/contextmenu'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import type { ComputedRef, Ref } from 'vue'

// Hai menu của dòng bảng, cùng danh sách mục (useRowMenu): menu chuột phải và menu bật từ nút
// ☰ cuối dòng. Đặt một lần cho mỗi bảng
const props = defineProps<{
  menu: {
    items: ComputedRef<MenuItem[]>
    context: Ref<InstanceType<typeof ContextMenu> | undefined>
    popup: Ref<InstanceType<typeof Menu> | undefined>
  }
}>()
const setContext = (el: unknown) => (props.menu.context.value = (el ?? undefined) as InstanceType<typeof ContextMenu> | undefined)
const setPopup = (el: unknown) => (props.menu.popup.value = (el ?? undefined) as InstanceType<typeof Menu> | undefined)
</script>

<template>
  <ContextMenu :ref="setContext" :model="menu.items.value" />
  <Menu :ref="setPopup" :model="menu.items.value" popup />
</template>
