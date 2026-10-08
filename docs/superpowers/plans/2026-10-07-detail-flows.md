# Detail Flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Asset types, statuses, accounts, roles and export profiles are opened, edited and changed the same way, with as few clicks as possible. That covers the detail pages, in-place editing with a save bar, create-then-open, and quick edit from lists, assets included.

**Architecture:**
- **Pure helpers, unit-tested:**
  - `lib/detailDraft.ts` holds unsaved edits per tab.
  - `lib/clickOrDouble.ts` tells a single click from a double click.
  - `features/assets/quickEdit.ts` builds full-record asset saves.
- **Shared components, built on top:** `OverviewFields`, `InlineCell`, `QuickEditDrawer`, plus extended `SaveBar` and `DetailHeader`.
- **Per-area save composables:** a `use…OverviewSave` for each area, so the detail page, the drawer and inline cells share one save-with-Undo path.
- **Then each area is migrated,** the report dialog's editing panes are extracted into `ReportEditor`, and the assets list gets inline cells and the drawer.

**Tech Stack:** Vue 3.5, TypeScript, PrimeVue 4.5 (Drawer, DatePicker, Select, InputText, Textarea), TanStack Vue Query 5, Vitest (node, no DOM).

**Spec:** `docs/superpowers/specs/2026-10-07-detail-flows-design.md`. One deviation: the export profile page has Columns, Layout and Format tabs, matching the report editor's three panes. The spec names only Columns and Format. It builds on `docs/superpowers/specs/2026-10-07-ui-patterns-design.md`:
- `runAction` and `announce` (`lib/actions.ts`);
- `confirmAction`, `confirmDiscard`, `mayClose` and `closeGuard` (`lib/confirm.ts`);
- `FormDialog`, `useDirty`, `notify`, `AppIcon` and the toasts.

## Global Constraints

- Code comments in Vietnamese; UI text in English. LF line endings (Python edits use `newline=''`).
- Build UI from PrimeVue v4 components.
- No one-sided coloured borders. Red only for delete and discard.
- No Edit buttons and no Edit dialogs. Fields are editable in place.
- No confirmation before anything Undo can reverse. Discard never asks. Leaving with unsaved edits asks.
- Every save goes through `runAction` or `announce`, using `toast: false` mutations, and offers Undo.
- Header action order, left to right:
  1. a related link or the main action;
  2. state changes;
  3. "More" (⋯);
  4. Delete (outlined danger), last.
- Locked fields show as text with a lock icon and a reason, never as disabled inputs.
- Read-only viewers see text only, no manage actions, and no save bar.
- Single click on an editable cell waits 220 ms before opening the page. Ctrl/⌘ click and middle click open at once.
- Vitest runs without a DOM, so only pure TypeScript is unit-tested.
- The gate for every task is `npm run check`, run in `frontend/`.
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Stage explicit paths.

## Review Focus

These five behaviours are most likely to bite someone, and no task's tests cover them. Each line gets its test in the task named.

1. **Editing a field back to its saved value** must not leave the save bar showing "1 unsaved change". *Test:* `setEdit` removes equal values (Task 1).
2. **Discard followed by Undo** brings the edits back without dropping edits made on another tab in between. *Test:* `restoreTab` merges into whatever is there (Task 1).
3. **Toggling Manage on a permission also ticks View,** through `catalog.toggle`. The save bar must count both changes, and unticking both must clear the draft. *Test:* `setList` and `listOf` (Task 1).
4. **A double click on an editable cell** must never navigate. A Ctrl-click must open at once, without waiting. *Test:* `clickOrDouble` (Task 2).
5. **An asset quick save** must send every other field unchanged, so the server's full replace doesn't wipe the description or attributes. Its Undo must send the full previous body. *Test:* `quickAssetBody` (Task 2).

---

### Task 1: Draft model

**Files:**
- Create: `frontend/src/lib/detailDraft.ts`
- Test: `frontend/src/lib/detailDraft.test.ts`

**Interfaces:**
- Produces:
  - `type DraftValue = string | number | boolean | null`;
  - `type Draft = Record<string, Record<string, DraftValue>>`;
  - `emptyDraft(): Draft`;
  - `setEdit(d, tab, key, value, saved): void`;
  - `valueOf(d, tab, key, saved): DraftValue`;
  - `changeCount(d, tab): number`;
  - `isDirty(d): boolean`;
  - `changesOf(d, tab): Record<string, DraftValue>`;
  - `discardTab(d, tab): Record<string, DraftValue>`;
  - `restoreTab(d, tab, edits): void`;
  - `clearTab(d, tab): void`;
  - `previousOf(saved, changes): Record<string, DraftValue>`;
  - `setList(d, tab, saved: string[], next: string[]): void`;
  - `listOf(d, tab, saved: string[], all: string[]): string[]`.

- [ ] **Step 1: Write the failing test** — `frontend/src/lib/detailDraft.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import {
  changeCount, changesOf, clearTab, discardTab, emptyDraft, isDirty, listOf, previousOf, restoreTab, setEdit, setList, valueOf,
} from './detailDraft'

describe('detailDraft', () => {
  it('counts edits per tab and drops an edit set back to the saved value', () => {
    const d = emptyDraft()
    setEdit(d, 'overview', 'name', 'Kế toán 2', 'Kế toán')
    setEdit(d, 'overview', 'description', 'x', '')
    expect(changeCount(d, 'overview')).toBe(2)
    expect(valueOf(d, 'overview', 'name', 'Kế toán')).toBe('Kế toán 2')
    setEdit(d, 'overview', 'name', 'Kế toán', 'Kế toán')
    expect(changeCount(d, 'overview')).toBe(1)
    expect(valueOf(d, 'overview', 'name', 'Kế toán')).toBe('Kế toán')
    expect(isDirty(d)).toBe(true)
    expect(changesOf(d, 'overview')).toEqual({ description: 'x' })
  })

  it('discards a tab and Undo merges the edits back without losing other tabs', () => {
    const d = emptyDraft()
    setEdit(d, 'overview', 'name', 'B', 'A')
    const removed = discardTab(d, 'overview')
    expect(isDirty(d)).toBe(false)
    setEdit(d, 'permissions', 'x', true, false)
    setEdit(d, 'overview', 'description', 'd', '')
    restoreTab(d, 'overview', removed)
    expect(changesOf(d, 'overview')).toEqual({ description: 'd', name: 'B' })
    expect(changeCount(d, 'permissions')).toBe(1)
    clearTab(d, 'overview')
    expect(changeCount(d, 'overview')).toBe(0)
  })

  it('gives the saved values of the changed keys for Undo', () => {
    expect(previousOf({ name: 'A', description: 'old', code: 'LAP' }, { name: 'B', description: 'new' })).toEqual({ name: 'A', description: 'old' })
  })

  it('tracks tick lists key by key', () => {
    const d = emptyDraft()
    const saved = ['assets.read']
    // Manage kéo theo View: cả hai cùng đổi
    setList(d, 'permissions', saved, ['assets.read', 'types.read', 'types.manage'])
    expect(changeCount(d, 'permissions')).toBe(2)
    expect(listOf(d, 'permissions', saved, ['assets.read', 'types.read', 'types.manage', 'roles.read'])).toEqual(['assets.read', 'types.read', 'types.manage'])
    setList(d, 'permissions', saved, ['assets.read'])
    expect(changeCount(d, 'permissions')).toBe(0)
  })
})
```

- [ ] **Step 2: Run it to see it fail**

Run: `npx vitest run src/lib/detailDraft.test.ts`
Expected: FAIL, because `./detailDraft` can't be resolved.

- [ ] **Step 3: Implement** — `frontend/src/lib/detailDraft.ts`:

```ts
// Bản nháp của trang chi tiết (và ngăn kéo sửa nhanh): thay đổi chưa lưu theo từng tab.
// Overview: trường → giá trị; danh sách tick (quyền, role): khoá → có/không. Thay đổi trùng
// giá trị đã lưu thì bỏ, nên số thay đổi luôn đúng. Thuần, để thử được; trang bọc bằng reactive()
export type DraftValue = string | number | boolean | null
export type Draft = Record<string, Record<string, DraftValue>>

export function emptyDraft(): Draft {
  return {}
}

export function setEdit(d: Draft, tab: string, key: string, value: DraftValue, saved: DraftValue) {
  const t = (d[tab] ??= {})
  if (value === saved) delete t[key]
  else t[key] = value
}

export function valueOf(d: Draft, tab: string, key: string, saved: DraftValue): DraftValue {
  const t = d[tab]
  return t && key in t ? t[key] : saved
}

export const changeCount = (d: Draft, tab: string) => Object.keys(d[tab] ?? {}).length
export const isDirty = (d: Draft) => Object.values(d).some((t) => Object.keys(t).length > 0)
export const changesOf = (d: Draft, tab: string): Record<string, DraftValue> => ({ ...(d[tab] ?? {}) })

// discardTab: bỏ thay đổi của tab, trả lại để Undo đặt lại
export function discardTab(d: Draft, tab: string): Record<string, DraftValue> {
  const removed = changesOf(d, tab)
  delete d[tab]
  return removed
}

// restoreTab: Undo của Discard; gộp vào thay đổi đang có (đã sửa thêm sau Discard thì giữ)
export function restoreTab(d: Draft, tab: string, edits: Record<string, DraftValue>) {
  d[tab] = { ...edits, ...(d[tab] ?? {}) }
}

// clearTab: sau khi lưu xong
export function clearTab(d: Draft, tab: string) {
  delete d[tab]
}

// previousOf: giá trị đã lưu của các khoá vừa đổi; Undo của Save gửi lại những giá trị này
export function previousOf(saved: Record<string, DraftValue>, changes: Record<string, DraftValue>): Record<string, DraftValue> {
  return Object.fromEntries(Object.keys(changes).map((k) => [k, saved[k] ?? null]))
}

// setList: danh sách tick mới so với danh sách đã lưu, ghi từng khoá
export function setList(d: Draft, tab: string, saved: string[], next: string[]) {
  for (const k of new Set([...saved, ...next])) setEdit(d, tab, k, next.includes(k), saved.includes(k))
}

// listOf: danh sách tick hiện tại (đã lưu + bản nháp), theo thứ tự của all
export function listOf(d: Draft, tab: string, saved: string[], all: string[]): string[] {
  return all.filter((k) => valueOf(d, tab, k, saved.includes(k)) === true)
}
```

- [ ] **Step 4: Run it to see it pass**

Run: `npx vitest run src/lib/detailDraft.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/detailDraft.ts frontend/src/lib/detailDraft.test.ts
git commit -m "feat(frontend): detail draft model (edits per tab, discard/restore, tick lists)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Click-or-double helper and asset quick save body

**Files:**
- Create:
  - `frontend/src/lib/clickOrDouble.ts`
  - `frontend/src/features/assets/quickEdit.ts`
- Test:
  - `frontend/src/lib/clickOrDouble.test.ts`
  - `frontend/src/features/assets/quickEdit.test.ts`

**Interfaces:**
- Produces:
  - `clickOrDouble(delay = 220): { click(e: ClickLike, open: () => void): void; double(): void; cancel(): void }`, where `ClickLike = { ctrlKey: boolean; metaKey: boolean; button: number }`;
  - `type AssetQuickChange = Partial<Pick<AssetBody, 'name' | 'status_id' | 'purchase_date' | 'description' | 'attributes'>>`;
  - `quickAssetBody(a: AssetDetail, change: AssetQuickChange): AssetBody`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/clickOrDouble.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { clickOrDouble } from './clickOrDouble'

const plain = { ctrlKey: false, metaKey: false, button: 0 }
afterEach(() => vi.useRealTimers())

describe('clickOrDouble', () => {
  it('opens after the delay on a single click', () => {
    vi.useFakeTimers()
    const open = vi.fn()
    const c = clickOrDouble(220)
    c.click(plain, open)
    vi.advanceTimersByTime(219)
    expect(open).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(open).toHaveBeenCalledOnce()
  })
  it('never opens when a double click follows', () => {
    vi.useFakeTimers()
    const open = vi.fn()
    const c = clickOrDouble(220)
    c.click(plain, open)
    c.click(plain, open)
    c.double()
    vi.advanceTimersByTime(1000)
    expect(open).not.toHaveBeenCalled()
  })
  it('opens at once on Ctrl/⌘ click and middle click', () => {
    const open = vi.fn()
    const c = clickOrDouble(220)
    c.click({ ...plain, ctrlKey: true }, open)
    c.click({ ...plain, metaKey: true }, open)
    c.click({ ...plain, button: 1 }, open)
    expect(open).toHaveBeenCalledTimes(3)
  })
})
```

`frontend/src/features/assets/quickEdit.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { AssetDetail } from '@/lib/api/types'
import { quickAssetBody } from './quickEdit'
import { assetBodyOf } from './values'

const a = {
  id: 'x', tag: 'CHR-0023', name: 'Ghế họp', description: 'Broken armrest.', version: 7,
  asset_type: { id: 't1' }, status: { id: 's-repair' }, location_id: 'l1', purchase_date: '2026-03-25',
  attributes: [{ key: 'color', label: 'Màu', data_type: 'text', value: 'Xám' }],
} as unknown as AssetDetail

describe('quickAssetBody', () => {
  it('changes only what was edited and keeps every other field for the full replace', () => {
    const body = quickAssetBody(a, { status_id: 's-available' })
    expect(body).toEqual({ ...assetBodyOf(a), status_id: 's-available' })
    expect(body.description).toBe('Broken armrest.')
    expect(body.attributes).toEqual({ color: 'Xám' })
  })
})
```

- [ ] **Step 2: Run them to see them fail**

Run: `npx vitest run src/lib/clickOrDouble.test.ts src/features/assets/quickEdit.test.ts`
Expected: FAIL, because the modules can't be resolved.

- [ ] **Step 3: Implement**

`frontend/src/lib/clickOrDouble.ts`:

```ts
// Ô sửa được trong danh sách: nhấp một lần mở trang chi tiết, nhấp đúp sửa tại chỗ. Nhấp
// một lần đợi một chút xem có nhấp đúp không; Ctrl/⌘ và chuột giữa (mở tab mới) mở ngay
export interface ClickLike {
  ctrlKey: boolean
  metaKey: boolean
  button: number
}

export function clickOrDouble(delay = 220) {
  let timer: ReturnType<typeof setTimeout> | undefined
  return {
    click(e: ClickLike, open: () => void) {
      clearTimeout(timer)
      if (e.ctrlKey || e.metaKey || e.button === 1) {
        open()
        return
      }
      timer = setTimeout(open, delay)
    },
    double() {
      clearTimeout(timer)
    },
    cancel() {
      clearTimeout(timer)
    },
  }
}
```

`frontend/src/features/assets/quickEdit.ts`:

```ts
// Sửa nhanh tài sản từ danh sách (ô sửa tại chỗ, ngăn kéo): PUT thay toàn bộ nên dựng body
// từ tài sản đọc mới nhất rồi chỉ đổi phần vừa sửa
import type { AssetDetail } from '@/lib/api/types'
import type { AssetBody } from './api'
import { assetBodyOf } from './values'

export type AssetQuickChange = Partial<Pick<AssetBody, 'name' | 'status_id' | 'purchase_date' | 'description' | 'attributes'>>

export function quickAssetBody(a: AssetDetail, change: AssetQuickChange): AssetBody {
  return { ...assetBodyOf(a), ...change }
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `npx vitest run src/lib/clickOrDouble.test.ts src/features/assets/quickEdit.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/clickOrDouble.ts frontend/src/lib/clickOrDouble.test.ts frontend/src/features/assets/quickEdit.ts frontend/src/features/assets/quickEdit.test.ts
git commit -m "feat(frontend): click-or-double helper and asset quick-save body

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Shared components

**Files:**
- Create:
  - `frontend/src/components/OverviewFields.vue`
  - `frontend/src/components/InlineCell.vue`
  - `frontend/src/components/QuickEditDrawer.vue`
- Modify:
  - `frontend/src/components/SaveBar.vue`
  - `frontend/src/components/DetailHeader.vue`
  - `frontend/src/app/tabs/tabPage.ts`

**Interfaces:**
- Consumes: Task 1 (`Draft`, `setEdit`, `valueOf`); Task 2 (`clickOrDouble`); `closeGuard` and `confirmDiscard` (`lib/confirm.ts`); `AppIcon`; `IconName`.
- Produces:
  - **`FieldDef`** (exported from `OverviewFields.vue` via `<script lang="ts">`):

    ```ts
    { key: string; label: string; kind?: 'text' | 'textarea' | 'select' | 'date'; options?: { label: string; value: string }[]; lock?: string; maxlength?: number }
    ```

  - **`<OverviewFields>`:**
    - props: `fields: FieldDef[]`, `saved: Record<string, DraftValue>`, `draft: Draft`, `tab?: string` (default `'overview'`), `readonly?: boolean`, `errors?: Record<string, string>`, `stacked?: boolean` (one column, for the drawer).
    - It mutates `draft` through `setEdit`.
  - **`<InlineCell>`:**
    - props: `value: string`, `editable: boolean`, `kind?: 'text' | 'select' | 'date'`, `options?: { label: string; value: string }[]`, `display?: string`.
    - emits: `save(value: string)`, `open(e: MouseEvent)`.
    - default slot: how the cell looks when not editing.
  - **`<QuickEditDrawer>`:**
    - model: `visible`;
    - props: `title: string`, `icon: IconName`, `dirty: boolean`, `busy?: boolean`, `canPrev: boolean`, `canNext: boolean`;
    - emits: `save`, `prev`, `next`, `openPage`;
    - slots: default (fields), `meta`.
  - **`<SaveBar>`:**
    - new optional prop `count?: number`;
    - when `count` is given and `message` is not, the message is "N unsaved change(s)";
    - Ctrl/⌘ S emits `save` while the page is active.
  - **`<DetailHeader>`:** new optional prop `icon?: IconName`, a soft brand tile shown when there's no `media` slot.
  - **`useLeaveGuard(dirty: () => boolean)`**, in `tabPage.ts`.

- [ ] **Step 1: SaveBar**

Replace `frontend/src/components/SaveBar.vue`'s `<script setup>` with the following. The template keeps its markup, but `{{ message }}` becomes `{{ text }}`.

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import { computed, onActivated, onDeactivated, onMounted, onUnmounted } from 'vue'

// Thanh "chưa lưu" dính ở đáy trang khi có thay đổi: Discard, Save; Ctrl/⌘ S lưu. blocked:
// không lưu được (vd luật lock-out), lý do hiện ở chỗ khác trên trang
const props = withDefaults(defineProps<{ message?: string; count?: number; saveLabel?: string; saving?: boolean; blocked?: boolean }>(), {
  saveLabel: 'Save',
})
const emit = defineEmits<{ save: []; discard: [] }>()
const text = computed(() => props.message ?? `${props.count ?? 0} unsaved ${props.count === 1 ? 'change' : 'changes'}`)

// trang trong KeepAlive vẫn giữ thanh lưu khi tab khác đang mở: chỉ nghe phím khi trang đang hiện
let active = true
function onKey(e: KeyboardEvent) {
  if (!active || !(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 's') return
  e.preventDefault()
  if (!props.saving && !props.blocked) emit('save')
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
onActivated(() => (active = true))
onDeactivated(() => (active = false))
</script>
```

- [ ] **Step 2: DetailHeader icon**

In `frontend/src/components/DetailHeader.vue`:
- Import `AppIcon` and `type IconName`.
- Change the props to `defineProps<{ title: string; icon?: IconName }>()`.
- Replace `<slot name="media" />` with:

  ```vue
  <slot name="media"><span v-if="icon" class="glyph"><AppIcon :name="icon" /></span></slot>
  ```

- Add this style:

  ```css
  .glyph { display: grid; place-items: center; width: 2.9rem; height: 2.9rem; border-radius: 12px; background: var(--app-brand-soft); color: var(--app-brand-strong); font-size: 1.5rem; flex: none; }
  ```

- [ ] **Step 3: OverviewFields** — `frontend/src/components/OverviewFields.vue`:

```vue
<script lang="ts">
// FieldDef: một trường của Overview (trang chi tiết và ngăn kéo sửa nhanh)
export interface FieldDef {
  key: string
  label: string
  kind?: 'text' | 'textarea' | 'select' | 'date'
  options?: { label: string; value: string }[]
  // không đổi được: hiện chữ, ổ khoá và lý do
  lock?: string
  maxlength?: number
}
</script>

<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import { formatDate, fromDateString, toDateString } from '@/lib/dates'
import { type Draft, type DraftValue, setEdit, valueOf } from '@/lib/detailDraft'

// Lưới nhãn/giá trị; trường sửa được là ô nhập ngay từ đầu (không có nút Edit). Đổi giá trị
// ghi vào bản nháp; trường đã đổi tô nền cam nhạt. Chỉ xem: mọi trường là chữ
const props = withDefaults(
  defineProps<{ fields: FieldDef[]; saved: Record<string, DraftValue>; draft: Draft; tab?: string; readonly?: boolean; errors?: Record<string, string>; stacked?: boolean }>(),
  { tab: 'overview' },
)
const val = (f: FieldDef) => valueOf(props.draft, props.tab, f.key, props.saved[f.key] ?? '')
const changed = (f: FieldDef) => val(f) !== (props.saved[f.key] ?? '')
const set = (f: FieldDef, v: DraftValue) => setEdit(props.draft, props.tab, f.key, v ?? '', props.saved[f.key] ?? '')
const shown = (f: FieldDef) => {
  const v = String(val(f) ?? '')
  if (f.kind === 'select') return f.options?.find((o) => o.value === v)?.label ?? v
  if (f.kind === 'date') return v ? formatDate(v) : '—'
  return v || '—'
}
</script>

<template>
  <div :class="['overview-fields', { stacked }]">
    <template v-for="f in fields" :key="f.key">
      <template v-if="f.lock || readonly">
        <span class="lbl">{{ f.label }}</span>
        <div class="ro">
          <span>{{ shown(f) }}</span>
          <span v-if="f.lock" class="lock"><i class="pi pi-lock" aria-hidden="true" />{{ f.lock }}</span>
        </div>
      </template>
      <template v-else>
        <label :for="`of-${f.key}`" class="lbl">{{ f.label }}</label>
        <div class="cell">
          <Textarea v-if="f.kind === 'textarea'" :id="`of-${f.key}`" :model-value="String(val(f) ?? '')" auto-resize rows="2" :maxlength="f.maxlength" :class="{ changed: changed(f) }" fluid @update:model-value="(v) => set(f, v)" />
          <Select v-else-if="f.kind === 'select'" :input-id="`of-${f.key}`" :model-value="val(f)" :options="f.options" option-label="label" option-value="value" :class="{ changed: changed(f) }" fluid @update:model-value="(v) => set(f, v)" />
          <DatePicker v-else-if="f.kind === 'date'" :input-id="`of-${f.key}`" :model-value="fromDateString(String(val(f) ?? '')) ?? undefined" date-format="dd/mm/yy" show-button-bar :class="{ changed: changed(f) }" fluid @update:model-value="(v) => set(f, toDateString(v as Date | null) ?? '')" />
          <InputText v-else :id="`of-${f.key}`" :model-value="String(val(f) ?? '')" :maxlength="f.maxlength" :class="{ changed: changed(f) }" fluid @update:model-value="(v) => set(f, v ?? '')" />
          <small v-if="errors?.[f.key]" class="field-error">{{ errors[f.key] }}</small>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.overview-fields { display: grid; grid-template-columns: 10rem minmax(0, 1fr); gap: 0.85rem 1.2rem; align-items: start; max-width: 44rem; }
.overview-fields.stacked { grid-template-columns: minmax(0, 1fr); gap: 0.3rem; }
.overview-fields.stacked .lbl { padding-top: 0.6rem; }
.lbl { padding-top: 0.5rem; font-size: 0.85rem; color: var(--p-text-muted-color); }
.cell { display: flex; flex-direction: column; gap: 0.25rem; min-width: 0; }
.ro { padding-top: 0.5rem; display: flex; flex-wrap: wrap; gap: 0.5rem; align-items: center; color: var(--p-text-color); }
.lock { display: inline-flex; gap: 0.3rem; align-items: center; font-size: 0.8rem; color: var(--p-text-muted-color); }
.lock i { font-size: 0.75rem; }
.cell :deep(.changed), .cell :deep(.changed input) { background: var(--app-warn-soft); border-color: color-mix(in srgb, var(--app-warn) 55%, var(--app-line)); }
@media (max-width: 640px) { .overview-fields { grid-template-columns: minmax(0, 1fr); gap: 0.3rem; } }
</style>
```

Check that `lib/dates.ts` exports `formatDate`, `fromDateString` and `toDateString` with these shapes. `values.ts` imports all three. If `formatDate` expects a `Date`, call `formatDate(fromDateString(v)!)` instead.

- [ ] **Step 4: InlineCell** — `frontend/src/components/InlineCell.vue`:

```vue
<script setup lang="ts">
import DatePicker from 'primevue/datepicker'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { nextTick, onUnmounted, ref } from 'vue'
import { clickOrDouble } from '@/lib/clickOrDouble'
import { fromDateString, toDateString } from '@/lib/dates'

// Ô sửa tại chỗ trong danh sách: nhấp đúp (hay F2 khi ô đang focus) thành ô nhập; Enter hay
// bấm ra ngoài lưu, Esc huỷ. Nhấp một lần vẫn mở trang chi tiết (đợi 220 ms để phân biệt)
const props = withDefaults(defineProps<{ value: string; editable: boolean; kind?: 'text' | 'select' | 'date'; options?: { label: string; value: string }[] }>(), {
  kind: 'text',
})
const emit = defineEmits<{ save: [value: string]; open: [e: MouseEvent] }>()

const editing = ref(false)
const text = ref('')
const timer = clickOrDouble()
onUnmounted(() => timer.cancel())

function onClick(e: MouseEvent) {
  if (editing.value) return
  timer.click(e, () => emit('open', e))
}
async function start() {
  if (!props.editable) return
  timer.double()
  text.value = props.value
  editing.value = true
  await nextTick()
  const el = document.querySelector<HTMLInputElement>('.inline-cell .p-inputtext:focus, .inline-cell input')
  el?.focus()
  el?.select?.()
}
let done = false
function finish(keep: boolean) {
  if (done) return
  done = true
  editing.value = false
  const v = text.value.trim()
  if (keep && v && v !== props.value) emit('save', v)
  setTimeout(() => (done = false))
}
</script>

<template>
  <span
    class="inline-cell"
    :class="{ editable }"
    :tabindex="editable ? 0 : undefined"
    :title="editable ? 'Double-click to edit' : undefined"
    @click.stop="onClick"
    @dblclick.stop="start"
    @keydown.f2.prevent="start"
  >
    <template v-if="editing">
      <Select
        v-if="kind === 'select'"
        v-model="text"
        :options="options"
        option-label="label"
        option-value="value"
        size="small"
        auto-focus
        @change="finish(true)"
        @hide="finish(false)"
        @keydown.esc.stop="finish(false)"
        @click.stop
      />
      <DatePicker
        v-else-if="kind === 'date'"
        :model-value="fromDateString(text) ?? undefined"
        size="small"
        date-format="dd/mm/yy"
        @update:model-value="(d) => { text = toDateString(d as Date | null) ?? ''; finish(true) }"
        @blur="finish(false)"
        @keydown.esc.stop="finish(false)"
        @click.stop
      />
      <InputText v-else v-model="text" size="small" fluid @keydown.enter.prevent="finish(true)" @keydown.esc.stop="finish(false)" @blur="finish(true)" @click.stop />
    </template>
    <slot v-else />
  </span>
</template>

<style scoped>
.inline-cell { display: inline-flex; max-width: 100%; border-radius: 6px; }
.inline-cell.editable { cursor: text; }
.inline-cell.editable:hover { box-shadow: 0 0 0 1px var(--app-line); }
.inline-cell:focus-visible { outline: 2px solid var(--app-accent); outline-offset: 1px; }
</style>
```

- [ ] **Step 5: QuickEditDrawer** — `frontend/src/components/QuickEditDrawer.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import Drawer from 'primevue/drawer'
import { onMounted, onUnmounted } from 'vue'
import { closeGuard } from '@/lib/confirm'
import AppIcon from './AppIcon.vue'
import type { IconName } from './icons'

// Sửa nhanh từ danh sách: ngăn kéo bên phải (chuột ít phải di), cùng form với Overview của
// trang chi tiết. ↑/↓ (khi không đang gõ) sang dòng trước/sau; Ctrl/⌘ S lưu; còn thay đổi
// chưa lưu thì hỏi trước khi đóng (cha tự hỏi khi chuyển dòng)
const props = defineProps<{ title: string; icon: IconName; dirty: boolean; busy?: boolean; canPrev: boolean; canNext: boolean }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ save: []; prev: []; next: []; openPage: [] }>()

const mayClose = closeGuard()
async function requestClose() {
  if (await mayClose(props.dirty)) visible.value = false
}
function onKey(e: KeyboardEvent) {
  if (!visible.value) return
  const typing = (e.target as HTMLElement | null)?.closest?.('input, textarea, select, [contenteditable], .p-select-overlay, .p-datepicker-panel')
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    if (props.dirty && !props.busy) emit('save')
    return
  }
  if (typing) return
  if (e.key === 'ArrowDown' && props.canNext) { e.preventDefault(); emit('next') }
  if (e.key === 'ArrowUp' && props.canPrev) { e.preventDefault(); emit('prev') }
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Drawer
    :visible="visible"
    position="right"
    :modal="false"
    :show-close-icon="false"
    :pt="{ root: { class: 'quick-edit', style: 'width: min(26rem, 100vw)' }, header: { class: 'qe-header' }, content: { class: 'qe-body' }, footer: { class: 'qe-footer' } }"
    @update:visible="(v: boolean) => !v && requestClose()"
  >
    <template #header>
      <span class="qe-icon"><AppIcon :name="icon" /></span>
      <div class="qe-title">
        <h2>Quick edit</h2>
        <span class="qe-name">{{ title }}</span>
      </div>
      <Button icon="pi pi-times" rounded severity="secondary" class="qe-close" aria-label="Close" @click="requestClose" />
    </template>
    <div class="qe-meta"><slot name="meta" /></div>
    <slot />
    <p class="qe-hint">↑/↓ moves to the previous or next row.</p>
    <template #footer>
      <Button label="Open full page" icon="pi pi-arrow-right" icon-pos="right" text @click="emit('openPage')" />
      <span class="qe-grow" />
      <Button label="Save" :loading="busy" :disabled="!dirty" @click="emit('save')" />
    </template>
  </Drawer>
</template>

<style>
/* không scoped: Drawer dựng ngoài cây component */
.quick-edit .qe-header { display: flex; align-items: center; gap: 0.75rem; padding: 1rem 1.1rem; background: var(--app-soft); }
.quick-edit .qe-icon { color: var(--app-brand); font-size: 1.6rem; }
.quick-edit .qe-title { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.quick-edit .qe-title h2 { font-family: var(--app-display); font-size: 1.2rem; font-weight: 800; }
.quick-edit .qe-name { font-size: 0.85rem; color: var(--p-text-muted-color); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.quick-edit .qe-close { background: var(--p-content-background); color: var(--p-text-color); border: 0; box-shadow: 0 2px 8px rgb(15 23 42 / 0.15); }
.quick-edit .qe-body { display: flex; flex-direction: column; gap: 1rem; padding: 1.1rem; }
.quick-edit .qe-hint { margin: 0; font-size: 0.8rem; color: var(--p-text-muted-color); }
.quick-edit .qe-footer { display: flex; align-items: center; gap: 0.5rem; padding: 0.75rem 1.1rem; border-top: 1px solid var(--app-line); }
.quick-edit .qe-grow { flex: 1; }
</style>
```

- [ ] **Step 6: useLeaveGuard**

Append to `frontend/src/app/tabs/tabPage.ts`, adding `onBeforeRouteLeave` from `vue-router` and `confirmDiscard` from `@/lib/confirm` to its imports:

```ts
// useLeaveGuard: rời trang (link, breadcrumb, sidebar) khi còn thay đổi chưa lưu thì hỏi.
// Chuyển sang tab khác của app không hỏi (bản nháp vẫn giữ, tab có chấm)
export function useLeaveGuard(dirty: () => boolean) {
  const tabs = useTabs()
  const tabId = tabs.routeTabId
  onBeforeRouteLeave(async () => (tabs.activeId !== tabId || !dirty() ? true : confirmDiscard()))
}
```

- [ ] **Step 7: Check and commit**

Run: `npm run check`
Expected: pass. The new components aren't used yet; vue-tsc checks them.

```bash
git add frontend/src/components/OverviewFields.vue frontend/src/components/InlineCell.vue frontend/src/components/QuickEditDrawer.vue frontend/src/components/SaveBar.vue frontend/src/components/DetailHeader.vue frontend/src/app/tabs/tabPage.ts
git commit -m "feat(frontend): OverviewFields, InlineCell, QuickEditDrawer; save bar Ctrl+S, header icon, leave guard

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Per-area recipe (Tasks 4–9)

Each area task follows this recipe. The task names its fields, actions and wiring.

**A. `use<Area>OverviewSave()`**, in `features/<area>/overviewSave.ts`. It returns `(item, changes) => Promise<boolean>`:

```ts
return (item, changes) => runAction({
  run: () => update.mutateAsync({ id: item.id, /* version: item.version when the API needs it */, ...merged }),
  done: `${changes.name ?? item.name} saved.`,
  failed: `Couldn't save ${item.name}.`,
  undo: (res) => update.mutateAsync({ id: item.id, /* version: res.version */, ...previousOf(savedValues(item), changes) }),
  undone: `Changes to ${item.name} undone.`,
  undoFailed: `Couldn't undo the changes to ${item.name}.`,
})
```

Where the API's PATCH needs every field (asset types), `merged` is the item's saved values with `changes` on top. Otherwise it is just `changes`. The detail page, the quick edit drawer and inline renames all call this one function.

**B. Detail page**, using a reactive `emptyDraft()` per page:
- `DetailHeader :icon`, with tags and the facts line, and actions in the fixed order.
- The Overview tab is `<OverviewFields :fields :saved :draft :readonly="!canManage" />`.
- `<SaveBar v-if="canManage && changeCount(draft, tab)" :count="changeCount(draft, tab)" :saving @save @discard />`.
- Save:
  - Overview calls the area's save with `changesOf(draft, 'overview')`. Tick-list tabs call their own `runAction`.
  - On success, `clearTab(draft, tab)`.
- Discard:

  ```ts
  const removed = discardTab(draft, tab)
  notify.success('Changes discarded.', { undo: () => restoreTab(draft, tab, removed) })
  ```

- `useTabDirty(() => isDirty(draft))` and `useLeaveGuard(() => isDirty(draft))`.
- Remove any Edit dialog, inline Save button or Save panel the page had.

**C. List:**
- Wrap the name (and the asset cells) in `<InlineCell :value :editable="canManage" @save="(v) => save(item, { name: v })" @open="(e) => openItem(item, e)">`.
- Add a pencil `IconAction` (`icon="pi pi-pencil" label="Quick edit"`) as the first row action. It sets `quick.value = item`.
- Mount a `<QuickEditDrawer>` with `<OverviewFields stacked :draft="quickDraft">`.
  - Prev and next move through the list's current rows: `rows.indexOf(quick)` ± 1, guarded by `mayClose(isDirty(quickDraft))`.
  - Save calls the area's save and clears the tab.
  - `openPage` routes to the item.

---

### Task 4: Asset types

**Files:**
- Create: `frontend/src/features/asset-types/overviewSave.ts`
- Modify:
  - `frontend/src/features/asset-types/pages/TypeSettingsPage.vue`
  - `frontend/src/features/asset-types/pages/AssetTypesPage.vue`

**Interfaces:**
- Consumes: Tasks 1–3; `useUpdateAssetType`, `useArchiveAssetType`, `useRestoreAssetType` (`toast: false`).
- Produces: `useTypeOverviewSave(): (t: AssetType, changes: Record<string, DraftValue>) => Promise<boolean>`.

- [ ] **Step 1: Save composable** — `frontend/src/features/asset-types/overviewSave.ts`:

```ts
// Lưu tên, mô tả của loại (trang chi tiết, ngăn kéo, ô sửa tại chỗ); PATCH cần đủ trường và version
import type { AssetType } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import { type DraftValue, previousOf } from '@/lib/detailDraft'
import { useUpdateAssetType } from './api'

export function useTypeOverviewSave() {
  const update = useUpdateAssetType()
  return (t: Pick<AssetType, 'id' | 'name' | 'description' | 'version'>, changes: Record<string, DraftValue>) => {
    const saved = { name: t.name, description: t.description }
    const next = { ...saved, ...changes } as { name: string; description: string }
    return runAction({
      run: () => update.mutateAsync({ id: t.id, version: t.version, ...next }),
      done: `${next.name} saved.`,
      failed: `Couldn't save ${t.name}.`,
      undo: (res) => update.mutateAsync({ id: t.id, version: res.version, ...saved, ...previousOf(saved, changes) } as never),
      undone: `Changes to ${t.name} undone.`,
      undoFailed: `Couldn't undo the changes to ${t.name}. The saved version stays.`,
    })
  }
}
```

- [ ] **Step 2: TypeSettingsPage → detail page**

- **Header:** `<DetailHeader :title="type.name" icon="sitemap">`.
  - Remove the `#media` slot and its `codeMark` span.
  - Tags:
    - `<Tag :value="type.archived_at ? 'Archived' : 'Active'" :severity="type.archived_at ? 'secondary' : 'success'" />`;
    - Built-in when `type.is_system`.
  - Facts line: `` `Code ${type.code} · ${assetCount} assets · ${activeAttributes} attributes` ``.
    - `assetCount` comes from `useAssetTypes(false, true)` (`asset_count`).
    - `activeAttributes` is the type's attributes where `!removed`.
- **Actions:**
  - a text button "View N assets" (`:to="typeListLocation(type.id, listContext.views)"`);
  - then, when `canManage && !type.is_system`, Archive or Restore (outlined, not red), calling the existing `toggleArchived`.
- **Tabs:** Overview and Attributes, through `useUrlState` (`tab` query, default `overview`), the same way `AccountDetailPage` does it.
  - Overview: `<OverviewFields :fields="[{ key: 'name', label: 'Name', maxlength: 100 }, { key: 'code', label: 'Code', lock: 'Part of every asset tag, so it can’t change.' }, { key: 'description', label: 'Description', kind: 'textarea' }]" :saved="{ name: type.name, code: type.code, description: type.description }" :draft :readonly="!canManage" />`.
  - Attributes: the current Attributes panel content (toolbar, table, dialogs), without the Panel wrapper. "Add attribute" sits at the top right of the tab.
- **Save bar** per recipe B. Save calls `useTypeOverviewSave()(type, changesOf(draft, 'overview'))`, then `clearTab`.
- **Remove:**
  - the "General" panel and its form (`name`, `description` refs, `saveDetails`, `errors`, the 409 reload branch);
  - the "Archive this type" Panel.
- **Draft guards:** `useTabDirty`/`useLeaveGuard(() => isDirty(draft))`.
- **New type landing:** when the route has `?tab=attributes&new=1`, focus the "Add attribute" button on mount. Give it `id="add-attribute"` and call `document.getElementById('add-attribute')?.focus()` in `onMounted` after the type loads.

- [ ] **Step 3: AssetTypesPage**

- **Names, in both views:** wrap each card title and each table name cell in `InlineCell`. `@save` calls `useTypeOverviewSave()(t, { name: v })`. `@open` calls the existing open function with the event.
- **Row actions:** add the pencil (quick edit) and, for non-system types when `canManage`, Archive or Restore through `runAction`. Use the same messages and Undo as `TypeSettingsPage.toggleArchived`; extract them into `overviewSave.ts` as `useTypeArchive(): (t) => Promise<boolean>` and use that in both places.
- **Quick edit drawer:** fields Name and Description, plus Code locked. Use the recipe C wiring over the rows currently shown.
- **New type:** after create, `router.push({ path: settingsPath(t), query: { tab: 'attributes', new: '1' } })`.

- [ ] **Step 4: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/asset-types
git commit -m "feat(frontend): asset type detail page, inline rename, quick edit, row archive

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Statuses

**Files:**
- Create:
  - `frontend/src/features/statuses/overviewSave.ts`
  - `frontend/src/features/statuses/pages/StatusDetailPage.vue`
- Modify:
  - `frontend/src/app/routes.ts`
  - `frontend/src/app/routes.test.ts`
  - `frontend/src/features/statuses/pages/StatusesPage.vue`

**Interfaces:**
- Consumes: Tasks 1–3; `useStatuses`, `useUpdateStatus` (`toast: false`), `useArchiveStatus`, `useRestoreStatus`.
- Produces:
  - route `status` at `/statuses/:id`;
  - `useStatusOverviewSave()`;
  - `useStatusLifecycle(): { archive(s), restore(s), makeDefault(s, all) }`. These move out of `StatusesPage`, with the same messages.

- [ ] **Step 1: Failing route test**

Append to the `routes` describe in `frontend/src/app/routes.test.ts`:

```ts
  it('names the status and export profile pages', () => {
    expect(router.resolve('/statuses/abc').name).toBe('status')
    expect(router.resolve('/export-profiles/abc').name).toBe('export-profile')
  })
```

Run: `npx vitest run src/app/routes.test.ts`
Expected: FAIL. The new paths resolve to `not-found`.

- [ ] **Step 2: Routes**

In `frontend/src/app/routes.ts`, after the `statuses` route:

```ts
      {
        path: 'statuses/:id',
        name: 'status',
        component: () => import('@/features/statuses/pages/StatusDetailPage.vue'),
        props: true,
        meta: { title: 'Status', icon: 'pi pi-circle', perm: Perm.AssetRead },
      },
```

After the `export-profiles` route:

```ts
      {
        path: 'export-profiles/:id',
        name: 'export-profile',
        component: () => import('@/features/export-profiles/pages/ExportProfilePage.vue'),
        props: true,
        meta: { title: 'Export profile', icon: 'pi pi-file-export', perm: Perm.AssetExport },
      },
```

Create `frontend/src/features/export-profiles/pages/ExportProfilePage.vue` now as a placeholder, so the import resolves. Task 9 fills it in.

```vue
<script setup lang="ts">
// Trang profile export: làm ở Task 9 của kế hoạch detail flows
defineProps<{ id: string }>()
</script>

<template>
  <section />
</template>
```

Run: `npx vitest run src/app/routes.test.ts`
Expected: PASS.

- [ ] **Step 3: overviewSave.ts**

```ts
// Lưu tên status; đổi trạng thái (archive, restore, mặc định) có Undo
import type { Status } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import type { DraftValue } from '@/lib/detailDraft'
import { useArchiveStatus, useRestoreStatus, useUpdateStatus } from './api'
import { KIND_INFO } from './lanes'

export function useStatusOverviewSave() {
  const update = useUpdateStatus()
  return (s: Pick<Status, 'id' | 'name'>, changes: Record<string, DraftValue>) => {
    const name = String(changes.name ?? s.name)
    return runAction({
      run: () => update.mutateAsync({ id: s.id, name }),
      done: `${name} saved.`,
      failed: `Couldn't save ${s.name}.`,
      undo: () => update.mutateAsync({ id: s.id, name: s.name }),
      undone: `${s.name} renamed back.`,
      undoFailed: `Couldn't rename it back. It stays ${name}.`,
    })
  }
}

export function useStatusLifecycle() {
  const archive = useArchiveStatus()
  const restore = useRestoreStatus()
  const update = useUpdateStatus()
  return {
    archive: (s: Status) =>
      runAction({ run: () => archive.mutateAsync(s.id), done: `${s.name} archived.`, failed: `Couldn't archive ${s.name}.`, undo: () => restore.mutateAsync(s.id), undone: `${s.name} restored.`, undoFailed: `Couldn't restore ${s.name}. It is still archived.` }),
    restore: (s: Status) =>
      runAction({ run: () => restore.mutateAsync(s.id), done: `${s.name} restored.`, failed: `Couldn't restore ${s.name}.`, undo: () => archive.mutateAsync(s.id), undone: `${s.name} archived again.`, undoFailed: `Couldn't archive ${s.name} again. It stays available.` }),
    makeDefault(s: Status, all: Status[]) {
      const prev = all.find((x) => x.kind === s.kind && x.is_default && !x.archived_at)
      const kind = KIND_INFO[s.kind].label.toLowerCase()
      return runAction({
        run: () => update.mutateAsync({ id: s.id, make_default: true }),
        done: `${s.name} is now the default ${kind} status.`,
        failed: `Couldn't make ${s.name} the default.`,
        undo: prev ? () => update.mutateAsync({ id: prev.id, make_default: true }) : undefined,
        undone: prev && `${prev.name} is the default ${kind} status again.`,
        undoFailed: `Couldn't change the default back. ${s.name} is still the default.`,
      })
    },
  }
}
```

Then, in `StatusesPage.vue`, replace `makeDefault`, `archiveStatus` and `restoreStatus` with calls to `useStatusLifecycle()`.

- [ ] **Step 4: StatusDetailPage.vue**

The page reads the status from `useStatuses(true, true)` with `computed(() => data.value?.find((s) => s.id === props.id))`. When the list has loaded and the status isn't in it, it shows `EmptyState` ("This status doesn't exist.").

- **Breadcrumb:** Statuses / name.
- **Header:**
  - `DetailHeader icon="tag"`.
  - Tags:
    - kind: `<Tag :value="KIND_INFO[s.kind].label" :severity="kindSeverity(s.kind)" />`;
    - Default (success) when `is_default`;
    - Archived (secondary) when archived.
  - Facts: `` `${s.asset_count ?? 0} assets · ${position} of ${laneSize} in ${KIND_INFO[s.kind].label}` ``.
- **Actions, in order:**
  1. text "View N assets", linking to `/assets?status=<id>` (use the asset list's status query key from `features/assets/listQuery.ts`);
  2. "Make default", when `canManage && !s.is_default && !s.archived_at`;
  3. Archive (when not default and not archived) or Restore, outlined.
- **No tab bar.** Overview fields:
  - `[{ key: 'name', label: 'Name', maxlength: 100 }, { key: 'kind', label: 'Kind', lock: 'The kind decides how assets behave, so it can’t change.' }]`;
  - saved: `{ name: s.name, kind: KIND_INFO[s.kind].label }`.
- **Save bar and guards** per recipe B.
- **`?focus=name`:** focus and select `#of-name` on mount.

- [ ] **Step 5: StatusesPage**

- **Remove the Edit status `FormDialog`** and its state (`editing`, `editName`, `editErrors`, `editForm`, `openEdit`, `submitEdit`).
- **Status name cell:** wrap it in `InlineCell`.
  - `@save` calls `useStatusOverviewSave()(s, { name: v })`.
  - `@open` calls `openLocation(router, `/statuses/${s.id}`, e)`.
- **Row actions:** the pencil `IconAction` opens the quick edit drawer. Fields: Name, plus Kind locked.
- **Row click** on the lane table opens the status page (`onRowClick`, `clickable-row` class).
- **The edit (pencil) icon** that opened the dialog goes; the quick edit pencil replaces it.
- **Inline add** keeps working. After a successful create, call `notify.success` with:
  - `undo`: archive the new status (there's no delete);
  - `action`: `{ label: 'Open', run: () => router.push(`/statuses/${created.id}`) }`.

- [ ] **Step 6: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/app/routes.ts frontend/src/app/routes.test.ts frontend/src/features/statuses frontend/src/features/export-profiles/pages/ExportProfilePage.vue
git commit -m "feat(frontend): status detail page, inline rename, quick edit; routes for status and profile pages

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Accounts

**Files:**
- Create: `frontend/src/features/accounts/overviewSave.ts`
- Modify:
  - `frontend/src/features/accounts/pages/AccountDetailPage.vue`
  - `frontend/src/features/accounts/pages/AccountsPage.vue`
  - `frontend/src/features/accounts/components/CreateAccountDialog.vue`

**Interfaces:**
- Consumes: Tasks 1–3; `useUpdateAccount` (switch it to `toast: false`), `useAssignRoles`, `useAccountActions`.
- Produces:
  - `useAccountOverviewSave(): (a: Pick<Account, 'id' | 'name' | 'version'>, changes) => Promise<boolean>`;
  - `useAccountRolesSave(): (a: { id: string; name: string }, before: string[], next: string[]) => Promise<boolean>`.

- [ ] **Step 1: overviewSave.ts**

Uses `useUpdateAccount`, with its `toast` argument changed to `false` in `api.ts` (it was `false` already: keep).

```ts
// Lưu tên tài khoản (PATCH cần version) và role của tài khoản, có Undo
import type { Account } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import type { DraftValue } from '@/lib/detailDraft'
import { useAssignRoles, useUpdateAccount } from './api'

export function useAccountOverviewSave() {
  const update = useUpdateAccount()
  return (a: Pick<Account, 'id' | 'name' | 'version'>, changes: Record<string, DraftValue>) => {
    const name = String(changes.name ?? a.name)
    return runAction({
      run: () => update.mutateAsync({ id: a.id, name, version: a.version }),
      done: `${name} saved.`,
      failed: `Couldn't save ${a.name}.`,
      undo: (res) => update.mutateAsync({ id: a.id, name: a.name, version: res.version }),
      undone: `${a.name} renamed back.`,
      undoFailed: `Couldn't rename it back. It stays ${name}.`,
    })
  }
}

export function useAccountRolesSave() {
  const assign = useAssignRoles()
  return (a: { id: string; name: string }, before: string[], next: string[]) =>
    runAction({
      run: () => assign.mutateAsync({ id: a.id, roleIds: next }),
      done: `Roles of ${a.name} saved.`,
      failed: "Couldn't save the roles.",
      undo: () => assign.mutateAsync({ id: a.id, roleIds: before }),
      undone: 'Roles put back.',
      undoFailed: "Couldn't put the roles back. The new roles stay.",
    })
}
```

- [ ] **Step 2: AccountDetailPage**

- **Header:**
  - `DetailHeader icon="user-plus"`. Keep the avatar `#media` slot, which takes precedence over the icon.
  - Tags stay as they are.
  - Facts: email.
- **Actions, in order:**
  - Disable or Enable (outlined; Disable disabled for yourself, with its tooltip);
  - a "More" `Button icon="pi pi-ellipsis-h"` opening a PrimeVue `Menu` popup with:
    - Resend invitation (invited);
    - Send reset link (active);
    - Sign out everywhere.

  These call the existing `useAccountActions` methods. The Resend and Send reset buttons in the header go.
- **Overview:**
  - Replace the "Profile" panel's form with `<OverviewFields :fields="[{ key: 'name', label: 'Display name', maxlength: 200 }, { key: 'email', label: 'Email', lock: 'The sign-in address. Invite a new account to use another one.' }]" :saved="{ name: account.name, email: account.email }" :draft :readonly="!canManage" />`.
  - Remove the inline Save button, `saveName`, `name` and `nameErrors`.
  - The Sign-in panel stays read-only.
- **Roles tab:** move its draft onto `detailDraft`.
  - `current = listOf(draft, 'roles', saved, allRoleIds)`.
  - `pick(id, on)` calls `setList(draft, 'roles', saved, next)`.
  - Save calls `useAccountRolesSave()(account, saved, current)`, then `clearTab(draft, 'roles')`.
  - Keep the lockout check (`blocked`).
- **One `SaveBar`** for whichever tab is open, per recipe B. Overview saves through `useAccountOverviewSave`.
- **Guards:** `useTabDirty`/`useLeaveGuard(() => isDirty(draft))`.

- [ ] **Step 3: AccountsPage**

- **Name cell:** wrap the person's name in `InlineCell`. `PersonCell` shows name and email, so wrap only the name text: give `PersonCell` a `name` slot, or pass `:name` through. `@save` calls `useAccountOverviewSave()(a, { name: v })`.
- **Row actions:** add the pencil, which opens the drawer with:
  - Name;
  - Email, locked;
  - a roles tick list (`setList` on the drawer's own draft, tab `'roles'`).

  Save calls both saves for whichever tabs changed.
- **The Invite dialog** opens the new account after a successful create. `CreateAccountDialog` emits `created(account)`, and the page calls `router.push(`/accounts/${account.id}`)`. Keep the success toast.

- [ ] **Step 4: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/accounts frontend/src/components/PersonCell.vue
git commit -m "feat(frontend): account detail in place, more menu, inline rename, quick edit, invite opens the account

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Roles

**Files:**
- Create: `frontend/src/features/roles/overviewSave.ts`
- Modify:
  - `frontend/src/features/roles/pages/RoleDetailPage.vue`
  - `frontend/src/features/roles/pages/RolesPage.vue`

**Interfaces:**
- Consumes: Tasks 1–3; `useUpdateRole` (`toast: false`), `useSetRolePermissions`, `useDeleteRole`, `useRestoreRole`; `catalog.toggle`.
- Produces: `useRoleOverviewSave()`; `useRoleDelete(): (r: { id: string; name: string }, after?: () => unknown) => Promise<boolean>`.

- [ ] **Step 1: overviewSave.ts**

```ts
// Lưu tên, mô tả role (role hệ thống giữ tên); xoá role có Undo
import { runAction } from '@/lib/actions'
import { type DraftValue, previousOf } from '@/lib/detailDraft'
import { useDeleteRole, useRestoreRole, useUpdateRole } from './api'

export function useRoleOverviewSave() {
  const update = useUpdateRole()
  return (r: { id: string; name: string; description: string }, changes: Record<string, DraftValue>) => {
    const saved = { name: r.name, description: r.description }
    const name = String(changes.name ?? r.name)
    return runAction({
      run: () => update.mutateAsync({ id: r.id, ...(changes as { name?: string; description?: string }) }),
      done: `${name} saved.`,
      failed: `Couldn't save ${r.name}.`,
      undo: () => update.mutateAsync({ id: r.id, ...(previousOf(saved, changes) as { name?: string; description?: string }) }),
      undone: `Changes to ${r.name} undone.`,
      undoFailed: `Couldn't undo the changes to ${r.name}.`,
    })
  }
}

export function useRoleDelete() {
  const remove = useDeleteRole()
  const restore = useRestoreRole()
  return (r: { id: string; name: string }, after?: () => unknown) =>
    runAction({
      run: () => remove.mutateAsync(r.id),
      done: `${r.name} deleted.`,
      failed: `Couldn't delete ${r.name}.`,
      undo: () => restore.mutateAsync(r.id),
      undone: `${r.name} restored.`,
      undoFailed: `Couldn't restore ${r.name}. It stays deleted.`,
      after,
    })
}
```

- [ ] **Step 2: RoleDetailPage**

- **Header:** `DetailHeader icon="shield"`.
  - Facts: `` `${people} ${people === 1 ? 'person' : 'people'} · ${saved.length} permissions` ``.
  - Actions: only Delete (custom roles). It keeps the "still held by" note, then calls `useRoleDelete()(role, () => router.push('/roles'))`.
  - Remove "Edit details", the Edit role dialog, `saveDetails`, `openEdit`, and the `name`/`description` refs.
- **Tabs:** Overview, Permissions, People. Overview is new and first; the default tab becomes `overview`.
  - Overview: `<OverviewFields :fields="[{ key: 'name', label: 'Name', maxlength: 100, lock: role.is_system ? 'Built-in roles keep their name.' : undefined }, { key: 'description', label: 'Description', kind: 'textarea' }]" :saved="{ name: role.name, description: role.description }" :draft :readonly="!canManage" />`.
  - Permissions: replace the `draft` ref with `detailDraft`.
    - `current = listOf(draft, 'permissions', saved, ALL_PERMS)`.
    - `set(code, on)` calls `setList(draft, 'permissions', saved, toggle(current, code, on))`.
    - `changed` comes from the draft.
    - Save keeps `savePermissions` (`runAction`), then `clearTab(draft, 'permissions')`.
  - People: unchanged.
- **One `SaveBar`** per recipe B. Overview saves through `useRoleOverviewSave`.
- **Guards:** `useTabDirty`/`useLeaveGuard`.
- **`?tab=permissions`** (from New role) opens on Permissions.

- [ ] **Step 3: RolesPage**

- **Card titles:** wrap them in `InlineCell` (editable for custom roles when `canManage`).
- **Delete:** add an outlined danger Delete `IconAction` on custom role cards nobody holds. It calls `useRoleDelete()(r)`.
- **Quick edit drawer:** Name and Description.
- **New role dialog:**
  - preselect `copyFrom` to the Employee role id (`EMPLOYEE_ROLE_ID` from `../catalog`) when the user may grant all its permissions;
  - on success, `router.push({ path: `/roles/${role.id}`, query: { tab: 'permissions' } })`.

- [ ] **Step 4: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/roles
git commit -m "feat(frontend): role overview in place, card rename and delete, quick edit, new role lands on permissions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Extract ReportEditor

**Files:**
- Create: `frontend/src/features/assets/export/components/ReportEditor.vue`
- Modify: `frontend/src/features/assets/export/components/ReportDialog.vue`

**Interfaces:**
- Produces `<ReportEditor>`:
  - **props:**
    - `source: ExportLayout`: the layout to start from;
    - `sourceKey: string | number`: changes whenever a new layout is loaded, which reloads the editor;
    - `types`, `rows`: from `usePreviewData`;
    - `scopeLabel: string`;
    - `titleName?: string`;
    - `pane?: 'columns' | 'layout' | 'format'`, which hides the editor's own tab bar and shows only that pane.
  - **emits:** `update:current(layout: ExportLayout)` on every change, and once on load.
  - **exposes:** `current: ComputedRef<ExportLayout>`, `hasColumns`, `sheetCount`.

There's no behaviour change. This task moves code.

- [ ] **Step 1: Move the editing state and panes**

Move these from `ReportDialog.vue` into `ReportEditor.vue`, keeping their code as is:
- **State:** `layout`, `columns`, `options`, `autoTicked`, `eachTypeAttrs`, `sheetName`, `sheetMode`, `sheetsOptions`, `current`, `sheets`, `skipped`, `title`, `hasColumns`, `sheetCount`, and the `options` watcher.
- **Panes:** the configuration tabs (`Tabs` with Columns, Layout and Format) and the preview pane (`SheetPreview`, plus the line under it).

The editor's `load()` replaces the dialog's loading of a layout:

```ts
function load() {
  layout.value = cloneLayout(props.source)
  autoTicked.value = []
  columns.value = editorColumns(layout.value, options.value)
}
watch(() => props.sourceKey, load, { immediate: true })
watch(current, (c) => emit('update:current', c), { immediate: true })
```

When `pane` is set, hide the `TabList` and bind `Tabs :value="pane"`.

- [ ] **Step 2: ReportDialog uses it**

`ReportDialog.vue` keeps:
- the profile `Select`, `saved`, `loadedVersion`, `dirty`;
- the Save… popover, `saveHere`, `saveNew`;
- `download`;
- the footer.

It computes `source` as `p ? p.layout : { ...defaultReportLayout(), sheet_name: … }`, with `sourceKey` = `` `${profileId.value ?? 'none'}:${loadedVersion.value ?? 0}` ``. It holds `current` from `@update:current`.

`dirty`, `patch` and `download` read the dialog's `current`. The `.report .body` layout stays the same, with the editor placed where the panes were.

- [ ] **Step 3: Check, then commit**

Run: `npm run check`
Expected: pass. The existing layout, format and api tests cover the pure helpers.

Then open the report dialog in the browser (`npm run dev`, or the running app). Check that:
- picking a profile loads it;
- ticking a column updates the preview;
- "Add each type's own attributes" ticks attribute columns;
- Save… and Download still work.

```bash
git add frontend/src/features/assets/export/components/ReportEditor.vue frontend/src/features/assets/export/components/ReportDialog.vue
git commit -m "refactor(frontend): extract ReportEditor from the report dialog

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Export profiles

**Files:**
- Create: `frontend/src/features/export-profiles/overviewSave.ts`
- Modify:
  - `frontend/src/features/export-profiles/pages/ExportProfilePage.vue` (the placeholder from Task 5)
  - `frontend/src/features/export-profiles/pages/ExportProfilesPage.vue`

**Interfaces:**
- Consumes: Tasks 1–3 and 8; `useExportProfiles`, `useUpdateExportProfile(false)`, `useCreateExportProfile`, `useDeleteExportProfile`, `useRestoreExportProfile`, `useExport`, `usePreviewData`, `defaultReportLayout`, `normalizeLayout`.
- Produces: `useProfileOverviewSave()`, `useProfileLayoutSave()`, `useProfileShare()`, `useProfileDelete()`.

- [ ] **Step 1: overviewSave.ts**

Move `toggleShare` and `deleteProfile` here from `ExportProfilesPage.vue`, as `useProfileShare()` and `useProfileDelete()`, with the same messages. Add:

```ts
export function useProfileOverviewSave() {
  const update = useUpdateExportProfile(false)
  return (p: Pick<ExportProfile, 'id' | 'name' | 'version'>, changes: Record<string, DraftValue>) => {
    const name = String(changes.name ?? p.name)
    return runAction({
      run: () => update.mutateAsync({ id: p.id, version: p.version, name }),
      done: `${p.name} renamed to ${name}.`,
      failed: `Couldn't rename ${p.name}.`,
      undo: (next) => update.mutateAsync({ id: p.id, version: next.version, name: p.name }),
      undone: `Name put back to ${p.name}.`,
      undoFailed: `Couldn't put the name back. It stays ${name}.`,
    })
  }
}

export function useProfileLayoutSave() {
  const update = useUpdateExportProfile(false)
  return (p: Pick<ExportProfile, 'id' | 'name' | 'version' | 'layout'>, layout: ExportLayout) =>
    runAction({
      run: () => update.mutateAsync({ id: p.id, version: p.version, layout }),
      done: `${p.name} saved.`,
      failed: `Couldn't save ${p.name}.`,
      undo: (next) => update.mutateAsync({ id: p.id, version: next.version, layout: p.layout }),
      undone: `${p.name} put back as it was.`,
      undoFailed: `Couldn't put ${p.name} back. The saved version stays.`,
    })
}
```

- [ ] **Step 2: ExportProfilePage.vue**

The page reads the profile from `useExportProfiles(true)` by id, with an `EmptyState` when it's gone.
- **Header:**
  - `DetailHeader icon="file"`.
  - Tags: Shared (info) or Only you (secondary).
  - Facts: `` `By ${p.owner.name} · ${p.layout.columns.length} columns · ${p.layout.sheets === 'per_type' ? 'sheet per type' : 'one sheet'}` ``.
- **Actions, in order:**
  1. "Export now", filled, using `useExport().run({ mode: 'report', filters: {}, profile_id: p.id }, `${p.name}.xlsx`)`;
  2. Share or Make private (when `p.can_edit`);
  3. "More" with Duplicate, which creates `${p.name} (copy)` with the same layout and `shared: false` through `useCreateExportProfile`, then routes to it;
  4. Delete (when `p.can_edit`), which then routes to `/export-profiles`.
  - When `!p.can_edit`, the only action besides Export now is "Duplicate to change it".
- **Tabs:**
  - Overview: Name, plus Owner locked ("Profiles keep the person who made them.").
  - Columns, Layout and Format: each is `<ReportEditor :pane="tab" :source="p.layout" :source-key="p.version" … @update:current="(l) => (layoutDraft = l)" />`.
- **Preview data:** `usePreviewData`, with the all-assets scope `ExportProfilesPage` builds today. Move that scope into `features/export-profiles/allAssetsScope.ts` as `useAllAssetsScope()`, so both pages use it.
- **Layout dirty state:** `normalizeLayout(layoutDraft) !== normalizeLayout(p.layout)`. On the three layout tabs the `SaveBar` shows "Layout changed", saved by `useProfileLayoutSave()(p, layoutDraft)`. Discard reloads the editor: bump a `layoutKey` that is appended to `source-key`.
- **Read-only (`!p.can_edit`):** pass `readonly` to the Overview fields. The layout tabs still show the editor so people can see how the profile is set up, but with no save bar and no Save. Their changes only affect the preview.
- **Guards:** `useTabDirty`/`useLeaveGuard` over the Overview draft and layout dirty.
- **`?tab=columns`** (from New profile and Duplicate) opens on Columns.

- [ ] **Step 3: ExportProfilesPage**

- **Row click** opens `/export-profiles/:id`. It no longer opens the report dialog; remove that dialog from this page.
- **Name cell:** `InlineCell` (editable when `p.can_edit`) saving through `useProfileOverviewSave`. Remove `startRename`, `saveRename`, `cancelRename`, the rename input, and the Rename… menu item.
- **Row actions:** add the pencil (quick edit drawer: Name, Owner locked), plus Export, Share and Delete through the composables.
- **"New profile" button** in the page header opens a `FormDialog` (`icon="file"`, title "New export profile", action "Create profile") with one field, Name. It creates `{ name, shared: false, layout: defaultReportLayout() }`, then routes to `{ path: `/export-profiles/${np.id}`, query: { tab: 'columns' } }`.

- [ ] **Step 4: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/export-profiles
git commit -m "feat(frontend): export profile page (overview, columns, layout, format), inline rename, quick edit, new profile

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Assets list quick edit

**Files:**
- Modify:
  - `frontend/src/features/assets/useAssetActions.ts`
  - `frontend/src/features/assets/pages/AssetsPage.vue`

**Interfaces:**
- Consumes: Task 2 (`quickAssetBody`, `AssetQuickChange`); Task 3; `fetchAsset`, `useReplaceAsset` (`toast: false`), `assetBodyOf`.
- Produces: `useAssetActions().quickSave(row: { id: string; tag: string }, change: AssetQuickChange, what: string): Promise<boolean>`.

- [ ] **Step 1: quickSave**

Add to the object `useAssetActions()` returns, importing `useReplaceAsset`, `quickAssetBody`, `assetBodyOf`, `isApiError` and `notify`:

```ts
    // Sửa nhanh từ danh sách: đọc bản mới nhất, chỉ đổi phần vừa sửa, PUT với version; Undo đặt lại
    quickSave(row: { id: string; tag: string }, change: AssetQuickChange, what: string) {
      return runAction({
        run: async () => {
          const a = await fetchAsset(row.id)
          const saved = await replaceAsset.mutateAsync({ id: a.id, version: a.version, ...quickAssetBody(a, change) })
          return { before: assetBodyOf(a), saved }
        },
        done: `${row.tag} ${what}.`,
        failed: `Couldn't save ${row.tag}.`,
        undo: ({ before, saved }) => replaceAsset.mutateAsync({ id: row.id, version: saved.version, ...before }),
        undone: `${row.tag} changed back.`,
        undoFailed: `Couldn't change ${row.tag} back. Someone may have changed it since.`,
      })
    },
```

`const replaceAsset = useReplaceAsset()` goes next to the other hooks.

- [ ] **Step 2: AssetsPage cells**

For rows that aren't retired, when `canManage`:
- **Name:**

  ```vue
  <InlineCell :value="a.name" :editable="canManage && !a.retired_at" @save="(v) => actions.quickSave(a, { name: v }, `renamed to ${v}`)" @open="(e) => actions.open(a, e)">{{ a.name }}</InlineCell>
  ```

- **Status:** `kind="select"`.
  - Options: `statusOptions`, the active non-retired statuses as `{ label: name, value: id }`.
  - `@save="(v) => actions.quickSave(a, { status_id: v }, `set to ${labelOf(v)}`)"`.
  - The slot renders the existing status `Tag`.
- **Purchased:** `kind="date"`.
  - `@save="(v) => actions.quickSave(a, { purchase_date: v }, 'purchase date changed')"`.
  - The slot renders the existing formatted date.

- [ ] **Step 3: AssetsPage drawer**

- **Pencil:** the row's pencil (`IconAction icon="pi pi-pencil" label="Quick edit"`) sets `quick`.
- **Loading:** the drawer loads the full asset with `useAsset(() => quick.value?.id)`.
- **Fields:**
  - Name;
  - Tag, locked ("Tags never change.");
  - Status (select);
  - Purchase date (date);
  - Description (textarea);
  - one `OverviewFields` entry per active type attribute, using `kind: 'date'` for date attributes and `kind: 'select'` for selects (options from the attribute's active options). Other data types are text.

  The saved values come from `fromApiValues(asset.attributes, asset.attributes)`, with dates as `YYYY-MM-DD` strings.
- **Save:**

  ```ts
  actions.quickSave(quick.value, { ...pick(name, status_id, purchase_date, description), attributes: toApiValues(asset.attributes, mergedValues) }, 'saved')
  ```

  Then clear the draft.
- **Prev and next** move through `data.items`, guarded by `mayClose`.
- **Open full page** routes to `/assets/:id`.

- [ ] **Step 4: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/assets/useAssetActions.ts frontend/src/features/assets/pages/AssetsPage.vue
git commit -m "feat(frontend): asset list inline name/status/date and quick edit drawer

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Docs and click budget walkthrough

**Files:**
- Modify: `frontend/README.md`

- [ ] **Step 1: README**

Add a section "Detail pages and quick edit" after "Interaction patterns":

```markdown
## Detail pages and quick edit

Spec: `docs/superpowers/specs/2026-10-07-detail-flows-design.md`.

- **The detail page** is `DetailHeader` (`icon`, tags, facts, then actions: link or main action, state change, More, Delete), then tabs with Overview first.
- **Fields** are `OverviewFields` over a `detailDraft` (`lib/detailDraft.ts`). There are no Edit buttons.
  - `SaveBar :count` saves with Ctrl/⌘ S.
  - Discard offers Undo.
  - Guard pages with `useTabDirty` and `useLeaveGuard`.
- **Each area has one save path** (`use…OverviewSave` in `features/<area>/overviewSave.ts`), shared by the page, the quick edit drawer and inline cells.
- **Lists:**
  - `InlineCell` edits a name, or an asset's status or date, on double-click. A single click opens the page after 220 ms.
  - `QuickEditDrawer` (the pencil) shows the Overview fields; ↑/↓ moves between rows.
- **Asset quick saves** read the full asset and PUT it with its version (`features/assets/quickEdit.ts`).
```

- [ ] **Step 2: Full check**

Run: `npm run check`
Expected: pass.

- [ ] **Step 3: Click budget walkthrough**

Walk through these in the dev app, as an admin and as a read-only account:

| Task | Expected |
|---|---|
| Rename a role, status, type, account, profile, asset | Double-click the name, type, Enter. No page load. Toast with Undo. |
| Change an asset's status | Double-click the status, pick. |
| Archive a type | Row icon, with Undo. |
| Disable an account, share a profile | One click, with Undo. |
| Add a status | Type in its column, Enter. Toast with Undo and Open. |
| New type with its first attribute | New type, Enter, then Add attribute (focused), Enter. |
| New role from another role | New role (Employee preselected), Enter. Lands on Permissions. |
| Invite someone | Invite account, fill in, Enter. Opens the account. |

Also check:
- a single click on a name opens the page after a short pause, and Ctrl-click opens a new tab at once;
- the drawer's ↑/↓, and its Ctrl+S;
- leaving a page with unsaved edits asks first;
- switching app tabs doesn't ask;
- read-only viewers see text, with no pencils or save bar.

- [ ] **Step 4: Commit**

```bash
git add frontend/README.md
git commit -m "docs(frontend): detail pages and quick edit reference

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
