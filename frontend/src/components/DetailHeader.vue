<script setup lang="ts">
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import AppIcon from './AppIcon.vue'
import CodeTag from './CodeTag.vue'
import type { IconName } from './icons'

// Thẻ đầu trang chi tiết, một kiểu cho mọi trang chi tiết (tài sản, phiếu mượn, loại, trạng
// thái, tài khoản, vai trò, mẫu xuất): ô biểu tượng (hay hình đại diện), mã tự sinh trên tiêu
// đề, tiêu đề và nhãn, hành động bên phải theo thứ tự cố định (hành động chính, đổi trạng thái,
// More, Delete cuối cùng); dưới cùng là dải thông tin chính (3–4 ô nhãn / giá trị)
export interface HeaderFact {
  label: string
  value: string | number
  to?: RouteLocationRaw
}
defineProps<{ title: string; icon?: IconName; code?: string; facts?: HeaderFact[] }>()
</script>

<template>
  <header class="detail-header">
    <div class="detail-top">
      <slot name="media"><span v-if="icon" class="glyph"><AppIcon :name="icon" /></span></slot>
      <div class="detail-main">
        <CodeTag v-if="code" :code="code" class="detail-code" />
        <div class="detail-line">
          <h1>{{ title }}</h1>
          <slot name="tags" />
        </div>
        <slot />
      </div>
      <div v-if="$slots.actions" class="actions"><slot name="actions" /></div>
    </div>
    <dl v-if="facts?.length" class="detail-facts">
      <div v-for="f in facts" :key="f.label" class="fact">
        <dt>{{ f.label }}</dt>
        <dd>
          <RouterLink v-if="f.to" :to="f.to">{{ f.value }}</RouterLink>
          <template v-else>{{ f.value }}</template>
        </dd>
      </div>
    </dl>
  </header>
</template>

<style scoped>
.detail-header {
  padding: 1.1rem 1.25rem;
  margin-bottom: 1.1rem;
  border: 1px solid var(--app-hair);
  border-radius: 16px;
  background: var(--p-content-background);
  box-shadow: var(--app-card-shadow);
}
.detail-top {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 1rem;
}
.glyph {
  display: grid;
  place-items: center;
  width: 2.9rem;
  height: 2.9rem;
  border-radius: 13px;
  background: linear-gradient(160deg, var(--app-brand), var(--app-brand-strong));
  color: #fff;
  box-shadow: 0 4px 12px color-mix(in srgb, var(--app-brand) 35%, transparent);
  font-size: 1.4rem;
  flex: none;
}
.detail-main {
  flex: 1;
  min-width: 12rem;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 0.3rem;
  color: var(--p-text-muted-color);
}
.detail-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  color: var(--p-text-color);
}
/* dải thông tin chính: các ô cách nhau bằng vạch mảnh, nhãn nhỏ mờ trên giá trị đậm vừa */
.detail-facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
  gap: 0.75rem 0;
  margin: 1rem 0 0;
  padding-top: 0.9rem;
  border-top: 1px solid var(--app-hair);
}
.fact {
  min-width: 0;
  padding: 0 1rem;
  border-left: 1px solid var(--app-hair);
}
.fact:first-child {
  padding-left: 0;
  border-left: 0;
}
.fact dt {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--p-text-muted-color);
}
.fact dd {
  margin: 0.15rem 0 0;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fact dd a {
  text-decoration: none;
}
</style>
