<script setup lang="ts">
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import { computed, ref, watch } from 'vue'
import type { ExportLayout } from '@/lib/api/types'
import { formatCell } from '../format'
import type { PreviewSheet, TypeInfo } from '../layout'

// Bản xem trước như Excel: chữ cột, số dòng, tab sheet; 20 dòng đầu mỗi sheet
const props = defineProps<{ sheets: PreviewSheet[]; layout: ExportLayout; types: TypeInfo[]; title: string[] }>()
const active = ref(0)
const tabs = computed(() => [...props.sheets.map((s) => s.name), ...(props.layout.summary ? ['Summary'] : [])])
watch(tabs, (t) => {
  if (active.value >= t.length) active.value = 0
})
const isSummary = computed(() => props.layout.summary && active.value === props.sheets.length)
const sheet = computed(() => props.sheets[active.value])
const letter = (i: number) => {
  let s = ''
  for (let n = i + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s
  return s
}
const KINDS = ['available', 'in_use', 'unavailable', 'retired'] as const
const KIND_LABEL = { available: 'Available', in_use: 'In use', unavailable: 'Unavailable', retired: 'Retired' }
const summary = computed(() =>
  props.sheets
    .flatMap((s) => s.rows)
    .reduce<Record<string, Record<string, number>>>((acc, r) => {
      acc[r.asset_type_name] ??= {}
      acc[r.asset_type_name][r.status_kind] = (acc[r.asset_type_name][r.status_kind] ?? 0) + 1
      return acc
    }, {}),
)
</script>

<template>
  <div class="sheet">
    <div class="sheet-scroll">
      <table v-if="isSummary" class="xl h-bold freeze">
        <thead>
          <tr>
            <th class="corner" />
            <th v-for="i in 6" :key="i" class="ch">{{ letter(i - 1) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr class="hdr">
            <td class="rh">1</td>
            <td>Type</td>
            <td v-for="k in KINDS" :key="k">{{ KIND_LABEL[k] }}</td>
            <td>Total</td>
          </tr>
          <tr v-for="(byKind, name, n) in summary" :key="name">
            <td class="rh">{{ n + 2 }}</td>
            <td>{{ name }}</td>
            <td v-for="k in KINDS" :key="k" class="right">{{ byKind[k] ?? 0 }}</td>
            <td class="right">{{ Object.values(byKind).reduce((a, b) => a + b, 0) }}</td>
          </tr>
        </tbody>
      </table>
      <table v-else-if="sheet" class="xl" :class="[`h-${layout.header}`, { freeze: layout.freeze, stripes: layout.stripes }]">
        <thead>
          <tr>
            <th class="corner" />
            <th v-for="(_, i) in sheet.columns" :key="i" class="ch">{{ letter(i) }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(line, i) in title" :key="`t${i}`">
            <td class="rh">{{ i + 1 }}</td>
            <td :class="i === 0 ? 'title' : 'titlesub'" :colspan="sheet.columns.length">{{ line }}</td>
          </tr>
          <tr class="hdr">
            <td class="rh">{{ title.length + 1 }}</td>
            <td v-for="c in sheet.columns" :key="c.field">{{ c.header }}<span v-if="layout.filter" class="filter-arrow" aria-hidden="true"><i class="pi pi-caret-down" /></span></td>
          </tr>
          <tr v-for="(r, i) in sheet.rows.slice(0, 20)" :key="r.id" class="data">
            <td class="rh">{{ title.length + i + 2 }}</td>
            <td v-for="c in sheet.columns" :key="c.field" :class="formatCell(r, c.field, layout, types).align">
              {{ formatCell(r, c.field, layout, types).text }}
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty">No rows to preview.</p>
    </div>
    <Tabs v-model:value="active" class="sheet-tabs">
      <TabList>
        <Tab v-for="(t, i) in tabs" :key="t" :value="i">{{ t }}</Tab>
      </TabList>
    </Tabs>
  </div>
</template>

<style scoped>
.sheet {
  border: 1px solid var(--app-line);
  border-radius: 8px;
  background: var(--p-content-background);
  overflow: hidden;
  font: 13px/1.3 Calibri, Carlito, 'Segoe UI', sans-serif;
}
.sheet-scroll {
  overflow: auto;
  max-height: 26rem;
}
table.xl {
  border-collapse: collapse;
}
table.xl th,
table.xl td {
  border: 1px solid var(--app-line);
  padding: 0.18rem 0.45rem;
  white-space: nowrap;
  height: 1.55rem;
}
.rh,
.ch {
  background: var(--app-soft);
  color: var(--p-text-muted-color);
  font: 11px var(--app-body);
  text-align: center;
  position: sticky;
  z-index: 1;
}
.ch {
  top: 0;
}
.rh {
  left: 0;
  min-width: 2.2rem;
}
.corner {
  position: sticky;
  top: 0;
  left: 0;
  z-index: 2;
  background: var(--app-soft);
}
.title {
  font-weight: 700;
  font-size: 14px;
}
.titlesub {
  color: var(--p-text-muted-color);
  font-size: 12px;
}
.h-bold .hdr td,
.h-bold_fill .hdr td {
  font-weight: 700;
}
.h-bold_fill .hdr td {
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
}
.freeze .hdr td {
  border-bottom: 2px solid var(--p-text-muted-color);
}
.stripes tr.data:nth-of-type(even) td {
  background: var(--app-ground);
}
td.right {
  text-align: right;
  font-variant-numeric: tabular-nums;
}
td.center {
  text-align: center;
}
.filter-arrow {
  display: inline-grid;
  place-items: center;
  width: 0.95rem;
  height: 0.95rem;
  margin-left: 0.35rem;
  border: 1px solid var(--app-line);
  border-radius: 2px;
  color: var(--p-text-muted-color);
}
.filter-arrow .pi {
  font-size: 8px;
}
.sheet-tabs {
  border-top: 1px solid var(--app-line);
  background: var(--app-soft);
}
.sheet-tabs :deep(.p-tablist-tab-list) {
  background: transparent;
}
.empty {
  padding: 2rem;
  text-align: center;
  color: var(--p-text-muted-color);
}
</style>
