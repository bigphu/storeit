# UI Interaction Patterns (Frontend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every screen asks, reports and undoes the same way:
- actions that change data run at once and offer Undo;
- the few confirmations, the form dialogs, toasts, tags, colours, fonts and scrollbars follow one design.

**Architecture:**
- **Foundation, built first** (Tasks 1–6):
  - a palette in `base.css` that the PrimeVue preset maps to;
  - bundled fonts;
  - a notice model in `lib/notify.ts`, drawn by a custom toast card (`NoticeCard.vue`);
  - `runAction` and `announce` in `lib/actions.ts` for act-now-with-Undo;
  - `confirmAction` in `lib/confirm.ts` over PrimeVue ConfirmDialog;
  - `FormDialog.vue` over PrimeVue Dialog;
  - `AppIcon.vue` filled icons;
  - `EmptyState.vue` and `TableSkeleton.vue`.
- **Feature tasks** (Tasks 7–13) move each area's actions to the spec's action map and its dialogs to `FormDialog`.
- The backend restore endpoints already exist (Plan A, commits a798ca2..fc0789d).

**Tech Stack:** Vue 3.5, TypeScript, PrimeVue 4.5 (Aura preset via `@primeuix/themes`), TanStack Vue Query 5, Vitest 5 (node environment, no DOM), `@fontsource`.

**Spec:** `docs/superpowers/specs/2026-10-07-ui-patterns-design.md` (approved). Where it differs from what was built in Plan A:
- Attribute and option restores are recorded as `asset_type_updated` (`removed → active`).
- Only labels can clash on restore.

## Global Constraints

- Code comments in Vietnamese; UI text in English.
- Files use LF line endings. When scripting edits with Python, open files with `newline=''`.
- Build UI from PrimeVue v4 components, not hand-rolled controls. The exceptions are the SVG icon, the toast card body and the confirmation bubble, which PrimeVue has no component for.
- No component shows state with a coloured border or stripe on one side. Thin grey dividers between areas stay.
- Red (`danger`) only for deleting and discarding. Archive, disable and retire are never red.
- Success messages read "*Thing* *past verb*." (for example "LAP-0012 retired.", "3 assets retired.").
- Titles and buttons name the action with a verb and an object, never "Confirm" or "OK".
- Vitest runs in node with no DOM. Only pure TypeScript is unit-tested. Components are checked by `vue-tsc` and in the browser.
- Every mutation called through `runAction` or `announce` must be created with `toast: false` (the `toast` argument of the feature's mutation factory). Otherwise its error shows twice.
- Never stage the user's own uncommitted edits. Before a task touches `frontend/src/app/layouts/TabBar.vue`, `frontend/src/features/accounts/pages/AccountsPage.vue` or `frontend/src/features/assets/export/components/ReportDialog.vue`, run `git status --short <file>`. If the file shows ` M` with changes this plan did not make, stop and ask the user to commit or stash them.
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Stage explicit paths.
- Run all `npm` commands in `frontend/`. The gate for every task is `npm run check` (vue-tsc, Vitest, build).

## Dialog migration recipe

Tasks 7–13 move dialogs to `FormDialog` (from Task 5) with these steps. Each task names the dialog, its `size`, `icon`, `title`, `action` and what counts as `dirty`.

1. Replace `import Dialog from 'primevue/dialog'` with `import FormDialog from '@/components/FormDialog.vue'`. Drop the `Message` import if it was only used for the general error.
2. Track unsaved input with `useDirty` (from Task 5, `@/lib/forms`).
   - Add `const form = useDirty(() => ({ /* the dialog's field refs' .value */ }))`.
   - Call `form.reset()` right after the fields are filled when the dialog opens: in the existing `watch(visible, …)` or the `open…()` function.
3. In the template:
   - Replace `<Dialog …>` with `<FormDialog v-model:visible="…" size="…" icon="…" :title="…" action="…" :busy="<mutation>.isPending.value" :error="errors.general.value" :dirty="form.dirty.value" @submit="<submit fn>">`.
   - Remove the inner `<form …>` wrapper, the general-error `<Message>`, and the `.actions` row with the Submit and Cancel buttons. `FormDialog` renders the form, the error and the buttons.
   - Move any explanatory hint paragraph that sat next to the buttons into `<template #hint>…</template>`.
   - Keep field-level errors (`errors.fields.value.x`) under their fields.
4. A dialog with no submit action (a manager or viewer) omits `action`. The footer then shows only Close, and the body is a `<div>`, so it may contain its own `<form>`.
5. The `width` prop overrides `size` only where a task says so.

---

## Review Focus

These five behaviours are most likely to bite someone, and nothing else exercises them. Each has a test in the task named.

1. **Ctrl/⌘ Z while typing** in an input, textarea, select or contenteditable must undo text, not the last action. Ctrl+Shift+Z and Ctrl+Alt+Z must not trigger Undo either. *Test:* `isUndoShortcut` cases (Task 2).
2. **Two Undo toasts visible.** Ctrl+Z undoes the newest. Once that toast is gone, the next Ctrl+Z undoes the older one. A closed toast's undo never runs. *Test:* `latestUndo` after `forgetUndo` (Task 2).
3. **Undo triggered twice** (double-click, or the button plus Ctrl+Z before the card unmounts) must reverse the action exactly once. *Test:* `announce` undo-once case (Task 4).
4. **Bulk status change where some selected assets already had the target status.** The server skips those without bumping their version, so Undo must leave them alone and send the right version for the rest. *Test:* `statusGroups` (Task 7).
5. **Undo of a closed tab after the tab list changed** (tabs closed so the old index is past the end, or tabs pinned meanwhile) puts the tab back within bounds and never inside the pinned group. *Test:* `reopenTab` cases (Task 13).

---

### Task 1: Palette, fonts, scrollbars, solid tags, no one-sided edges

**Files:**
- Modify:
  - `frontend/package.json` (via npm install)
  - `frontend/src/main.ts`
  - `frontend/src/app/base.css`
  - `frontend/src/app/theme.ts`
  - `frontend/src/features/statuses/api.ts:14-22`
  - `frontend/src/features/accounts/status.ts`
  - `frontend/src/features/roles/pages/RoleDetailPage.vue` (style `tr.changed`)
  - `frontend/src/features/assets/export/components/ColumnEditor.vue` (style, lines ~113-115)
- Create: `frontend/src/lib/tones.ts`
- Test: `frontend/src/lib/tones.test.ts`

**Interfaces:**
- Produces:
  - CSS variables `--app-{brand|info|warn|danger|neutral}`, plus `-strong`, `-ink` and `-soft` for each;
  - `--app-on-color`, `--app-row-hover`, `--app-scroll-thumb`, `--app-scroll-thumb-hover`;
  - `type Tone = 'success' | 'info' | 'warn' | 'danger' | 'secondary'`;
  - `statusKindTone(k: StatusKind): Tone`, `accountStatusTone(s: Account['status']): Tone`.
- `kindSeverity` and `statusSeverity` keep their names and delegate.

- [ ] **Step 1: Write the failing test** — `frontend/src/lib/tones.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { accountStatusTone, statusKindTone } from './tones'

describe('tones', () => {
  it('maps status kinds to the spec tag roles', () => {
    expect(statusKindTone('available')).toBe('success')
    expect(statusKindTone('in_use')).toBe('info')
    expect(statusKindTone('unavailable')).toBe('warn')
    expect(statusKindTone('retired')).toBe('danger')
  })
  it('maps account statuses to the spec tag roles', () => {
    expect(accountStatusTone('active')).toBe('success')
    expect(accountStatusTone('invited')).toBe('info')
    expect(accountStatusTone('disabled')).toBe('danger')
  })
})
```

- [ ] **Step 2: Run it to see it fail**

Run: `npx vitest run src/lib/tones.test.ts`
Expected: FAIL, because `./tones` can't be resolved.

- [ ] **Step 3: Implement `lib/tones.ts` and delegate**

`frontend/src/lib/tones.ts`:

```ts
// Màu tag theo nghĩa (spec "Tags"): tốt/dùng được brand, đang dùng info, cần chú ý warn,
// ngừng dùng danger, không hoạt động neutral. Tên là severity của PrimeVue Tag
import type { Account, StatusKind } from '@/lib/api/types'

export type Tone = 'success' | 'info' | 'warn' | 'danger' | 'secondary'

export function statusKindTone(k: StatusKind): Tone {
  return ({ available: 'success', in_use: 'info', unavailable: 'warn', retired: 'danger' } as const)[k]
}

export function accountStatusTone(s: Account['status']): Tone {
  return s === 'active' ? 'success' : s === 'invited' ? 'info' : 'danger'
}
```

In `features/statuses/api.ts`, replace the body of `kindSeverity` (lines 14-22) with:

```ts
// Màu tag theo kind
export function kindSeverity(k: StatusKind): Tone {
  return statusKindTone(k)
}
```

Add `import { statusKindTone, type Tone } from '@/lib/tones'` to that file's imports.

Replace `features/accounts/status.ts` with:

```ts
import type { Account } from '@/lib/api/types'
import { accountStatusTone, type Tone } from '@/lib/tones'

// Màu tag theo trạng thái tài khoản
export function statusSeverity(s: Account['status']): Tone {
  return accountStatusTone(s)
}
```

- [ ] **Step 4: Run it to see it pass**

Run: `npx vitest run src/lib/tones.test.ts`
Expected: PASS (2 tests).

- [ ] **Step 5: Install the fonts and import them**

Run: `npm install @fontsource/be-vietnam-pro @fontsource-variable/bricolage-grotesque @fontsource/jetbrains-mono`

In `frontend/src/main.ts`, add these imports after `import 'primeicons/primeicons.css'`:

```ts
// Font đóng gói theo app (không tải từ Google): chữ, tiêu đề, mã
import '@fontsource/be-vietnam-pro/400.css'
import '@fontsource/be-vietnam-pro/500.css'
import '@fontsource/be-vietnam-pro/600.css'
import '@fontsource/be-vietnam-pro/700.css'
import '@fontsource-variable/bricolage-grotesque'
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/500.css'
import '@fontsource/jetbrains-mono/600.css'
```

- [ ] **Step 6: Palette, fonts and scrollbars in `base.css`**

Replace the header comment and the `:root { … }` and `:root.app-dark { … }` blocks at the top of `frontend/src/app/base.css` (lines 1-26) with:

```css
/* Bố cục và định dạng chung. Bảng màu thống nhất (spec "Colour") và font đóng gói
   (Be Vietnam Pro, Bricolage Grotesque, JetBrains Mono); preset PrimeVue trong theme.ts
   lấy màu từ đây. */

:root {
  --app-display: 'Bricolage Grotesque Variable', 'Be Vietnam Pro', system-ui, sans-serif;
  --app-body: 'Be Vietnam Pro', system-ui, 'Segoe UI', sans-serif;
  --app-mono: 'JetBrains Mono', ui-monospace, 'Cascadia Mono', Consolas, monospace;
  /* Mỗi vai trò bốn sắc: tươi (biểu tượng, bong bóng, tiến trình, thanh cuộn, focus), đậm
     (nền có chữ trắng: tag, nút đặc; tiêu đề màu), ink (chữ màu trên nền sáng), soft (hộp
     thông báo, vùng chọn). Tươi và đậm giống nhau ở hai theme. brand cũng là "thành công". */
  --app-brand: #10b981;
  --app-brand-strong: #059669;
  --app-brand-ink: #047857;
  --app-brand-soft: color-mix(in srgb, #10b981 14%, transparent);
  --app-info: #0ea5e9;
  --app-info-strong: #0284c7;
  --app-info-ink: #0369a1;
  --app-info-soft: color-mix(in srgb, #0ea5e9 14%, transparent);
  --app-warn: #f97316;
  --app-warn-strong: #ea580c;
  --app-warn-ink: #c2410c;
  --app-warn-soft: color-mix(in srgb, #f97316 14%, transparent);
  --app-danger: #ef4444;
  --app-danger-strong: #dc2626;
  --app-danger-ink: #b91c1c;
  --app-danger-soft: color-mix(in srgb, #ef4444 12%, transparent);
  --app-neutral: #94a3b8;
  --app-neutral-strong: #64748b;
  --app-neutral-ink: #475569;
  --app-neutral-soft: color-mix(in srgb, #64748b 12%, transparent);
  --app-on-color: #ffffff;
  /* dẫn xuất: luôn tính từ bảng màu */
  --app-row-hover: color-mix(in srgb, var(--app-brand) 8%, transparent);
  --app-scroll-thumb: color-mix(in srgb, var(--app-brand) 38%, transparent);
  --app-scroll-thumb-hover: color-mix(in srgb, var(--app-brand) 70%, transparent);
  /* nền dưới cùng (sau thanh trên, sidebar), màu nhấn, nền phụ */
  --app-ground: var(--p-surface-50);
  --app-soft: var(--p-surface-100);
  --app-line: var(--p-content-border-color);
  --app-accent: var(--app-brand);
  --app-link: var(--app-brand-ink);
  /* mục "đang ở đây" (sidebar, thẻ, dòng đang chọn): nền brand nhạt; rê chuột: xám */
  --app-selected: var(--app-brand-soft);
  --app-hover: var(--p-surface-100);
}
:root.app-dark {
  --app-ground: var(--p-surface-950);
  --app-soft: var(--p-surface-800);
  --app-hover: var(--p-surface-800);
  --app-brand-ink: #34d399;
  --app-brand-soft: color-mix(in srgb, #10b981 20%, transparent);
  --app-info-ink: #38bdf8;
  --app-info-soft: color-mix(in srgb, #0ea5e9 20%, transparent);
  --app-warn-ink: #fb923c;
  --app-warn-soft: color-mix(in srgb, #f97316 20%, transparent);
  --app-danger-ink: #f87171;
  --app-danger-soft: color-mix(in srgb, #ef4444 20%, transparent);
  --app-neutral-ink: #cbd5e1;
  --app-neutral-soft: color-mix(in srgb, #94a3b8 20%, transparent);
}

/* Thanh cuộn mảnh 6px, không rãnh, nút màu brand nhạt. Chrome bỏ qua ::-webkit-scrollbar
   khi có scrollbar-width/color, nên hai thuộc tính chuẩn chỉ dùng cho trình duyệt không có
   ::-webkit-scrollbar (Firefox) */
@supports not selector(::-webkit-scrollbar) {
  * {
    scrollbar-width: thin;
    scrollbar-color: var(--app-scroll-thumb) transparent;
  }
}
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}
::-webkit-scrollbar-track,
::-webkit-scrollbar-corner {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--app-scroll-thumb);
  border-radius: 999px;
}
::-webkit-scrollbar-thumb:hover {
  background: var(--app-scroll-thumb-hover);
}
/* vùng có thể bắt đầu hay thôi cuộn (thân hộp thoại, các ngăn của hộp thoại báo cáo):
   giữ sẵn chỗ để nội dung không nhảy */
.scroll-pane,
.p-dialog-content {
  scrollbar-gutter: stable;
}
```

Then make these edits in the same file:
- In the `h1, h2, h3` rule, change `font-weight: 600;` to `font-weight: 700;`.
- Delete the last rule of the file: the comment `/* Dòng đang chọn của bảng: … */` and the `.p-datatable-tbody > tr.p-datatable-row-selected > td:first-child { box-shadow: var(--app-bar-left); }` block. The selected-row background stays, and the left stripe goes.
- Delete `.p-tag { font-weight: 500; }`. Tag weight now comes from the preset.

- [ ] **Step 7: Map the preset to the palette, with solid tags**

In `frontend/src/app/theme.ts`:
- Replace `const rowHover = 'color-mix(in srgb, {primary.color} 8%, transparent)'` with `const rowHover = 'var(--app-row-hover)'`.
- Change the paginator's `selectedBackground` to `'var(--app-selected)'`.
- Before `export const StoreItPreset`, add:

```ts
// Tag đặc: nền là sắc đậm của vai trò, chữ trắng đậm, bo tròn (spec "Tags"); hai theme như nhau
const solidTags = {
  primary: { background: 'var(--app-brand-strong)', color: 'var(--app-on-color)' },
  success: { background: 'var(--app-brand-strong)', color: 'var(--app-on-color)' },
  info: { background: 'var(--app-info-strong)', color: 'var(--app-on-color)' },
  warn: { background: 'var(--app-warn-strong)', color: 'var(--app-on-color)' },
  danger: { background: 'var(--app-danger-strong)', color: 'var(--app-on-color)' },
  secondary: { background: 'var(--app-neutral-strong)', color: 'var(--app-on-color)' },
}
```

and inside `components: { … }` add:

```ts
    tag: {
      root: { fontSize: '0.76rem', fontWeight: '700', padding: '0.12rem 0.55rem', borderRadius: '999px' },
      colorScheme: { light: solidTags, dark: solidTags },
    },
```

- [ ] **Step 8: Remove the other one-sided edges**

- `RoleDetailPage.vue`, in its `<style>`: replace the `:deep(tr.changed > td:first-child) { box-shadow: var(--app-bar-left); }` rule and its comment with:

```css
/* dòng có thay đổi chưa lưu: nền cam nhạt (không vạch một bên) */
:deep(tr.changed > td) {
  background: var(--app-warn-soft);
}
```

- `ColumnEditor.vue`: replace the `.col-editor :deep(tr:has(.label:focus-visible)) > td:first-child { box-shadow: inset 2px 0 0 var(--p-primary-color); }` rule with:

```css
.label:focus-visible {
  outline: 2px solid var(--app-brand);
  outline-offset: 2px;
  border-radius: 4px;
}
```

- [ ] **Step 9: Check**

Run: `npm run check`
Expected: vue-tsc passes, all Vitest tests pass, and the build succeeds.

Then run `grep -rn "app-bar-left" src`. Expected: no output.

- [ ] **Step 10: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/main.ts frontend/src/app/base.css frontend/src/app/theme.ts frontend/src/lib/tones.ts frontend/src/lib/tones.test.ts frontend/src/features/statuses/api.ts frontend/src/features/accounts/status.ts frontend/src/features/roles/pages/RoleDetailPage.vue frontend/src/features/assets/export/components/ColumnEditor.vue
git commit -m "feat(frontend): unified palette, bundled fonts, thin scrollbars, solid tags

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Notice model, Undo registry, countdown

**Files:**
- Modify:
  - `frontend/src/lib/notify.ts` (rewrite)
  - `frontend/src/lib/notify.test.ts` (rewrite)
  - `frontend/src/features/assets/export/useExport.ts:24`
- Create:
  - `frontend/src/lib/countdown.ts`
  - `frontend/src/lib/countdown.test.ts`

**Interfaces:**
- Produces, from `lib/notify.ts`:
  - `type Severity`, `interface NoticeAction { label: string; run: () => void }`;
  - `interface Notice { id: number; severity: Severity; summary: string; detail?: string; action?: NoticeAction; undo?: () => void; retry?: () => void; life: number }`. A `life` of `0` means the notice stays until closed.
  - `notify.success(summary, { undo?, action? })`, `notify.info(summary, { action? })`, `notify.error(summary, { detail?, retry? })`. Each returns the `Notice` it emitted.
  - `onNotice(listener): () => void`;
  - `trackUndo(id, run)`, `forgetUndo(id)`, `latestUndo(): (() => void) | undefined`;
  - `isUndoShortcut(e): boolean`.
- Produces, from `lib/countdown.ts`: `countdown(life: number, start: number): { pause(now), resume(now), left(now): number }`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/notify.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'
import { forgetUndo, isUndoShortcut, latestUndo, type Notice, notify, onNotice, trackUndo } from './notify'

function capture() {
  const seen: Notice[] = []
  const off = onNotice((n) => seen.push(n))
  return { seen, off }
}

describe('notify', () => {
  it('passes options to listeners and picks the life', () => {
    const { seen, off } = capture()
    const run = vi.fn()
    notify.success('Downloaded a.xlsx.', { action: { label: 'What’s inside', run } })
    notify.success('LAP-1 retired.', { undo: () => {} })
    notify.success('Saved.')
    notify.error('Couldn’t save.', { detail: 'Name is taken.', retry: () => {} })
    off()
    expect(seen[0].action?.label).toBe('What’s inside')
    expect(seen[0].life).toBe(8000)
    expect(seen[1].undo).toBeTypeOf('function')
    expect(seen[1].life).toBe(8000)
    expect(seen[2].life).toBe(4000)
    expect(seen[3]).toMatchObject({ severity: 'error', summary: 'Couldn’t save.', detail: 'Name is taken.', life: 0 })
    expect(seen[3].retry).toBeTypeOf('function')
    expect(new Set(seen.map((n) => n.id)).size).toBe(4)
  })
})

describe('undo registry', () => {
  it('runs the newest visible undo, then the older one once the newest is gone', () => {
    const a = vi.fn()
    const b = vi.fn()
    trackUndo(1, a)
    trackUndo(2, b)
    latestUndo()?.()
    expect(b).toHaveBeenCalledOnce()
    forgetUndo(2)
    latestUndo()?.()
    expect(a).toHaveBeenCalledOnce()
    forgetUndo(1)
    expect(latestUndo()).toBeUndefined()
  })
})

describe('isUndoShortcut', () => {
  const key = (o: Partial<{ key: string; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean; altKey: boolean }>, target: unknown = null) =>
    ({ key: 'z', ctrlKey: false, metaKey: false, shiftKey: false, altKey: false, ...o, target }) as unknown as KeyboardEvent
  const field = { closest: () => ({}) }
  const plain = { closest: () => null }

  it('accepts Ctrl+Z and ⌘Z outside fields', () => {
    expect(isUndoShortcut(key({ ctrlKey: true }, plain))).toBe(true)
    expect(isUndoShortcut(key({ metaKey: true, key: 'Z' }, plain))).toBe(true)
    expect(isUndoShortcut(key({ ctrlKey: true }))).toBe(true)
  })
  it('ignores typing targets, redo and other keys', () => {
    expect(isUndoShortcut(key({ ctrlKey: true }, field))).toBe(false)
    expect(isUndoShortcut(key({ ctrlKey: true, shiftKey: true }, plain))).toBe(false)
    expect(isUndoShortcut(key({ ctrlKey: true, altKey: true }, plain))).toBe(false)
    expect(isUndoShortcut(key({ ctrlKey: true, key: 'y' }, plain))).toBe(false)
    expect(isUndoShortcut(key({}, plain))).toBe(false)
  })
})
```

`frontend/src/lib/countdown.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { countdown } from './countdown'

describe('countdown', () => {
  it('counts down, pauses and resumes', () => {
    const c = countdown(1000, 0)
    expect(c.left(250)).toBeCloseTo(0.75)
    c.pause(250)
    expect(c.left(900)).toBeCloseTo(0.75)
    c.pause(950) // tạm dừng lần nữa không đổi gì
    c.resume(900)
    expect(c.left(1150)).toBeCloseTo(0.5)
    c.resume(1150) // đang chạy: không đổi gì
    expect(c.left(5000)).toBe(0)
  })
})
```

- [ ] **Step 2: Run them to see them fail**

Run: `npx vitest run src/lib/notify.test.ts src/lib/countdown.test.ts`
Expected: FAIL. `trackUndo`, `isUndoShortcut` and `./countdown` don't exist yet, and the old `notify.success(s, action)` signature doesn't match.

- [ ] **Step 3: Implement**

`frontend/src/lib/notify.ts`:

```ts
// Thông báo dùng được ngoài component (QueryClient, session, runAction); App.vue vẽ bằng
// NoticeCard trong Toast. Thành công 4 giây, có Undo hay nút thì 8 giây, lỗi không tự đóng
export type Severity = 'success' | 'info' | 'warn' | 'error'

// Nút trong thông báo (vd "What’s inside" sau khi tải export dữ liệu)
export interface NoticeAction {
  label: string
  run: () => void
}

export interface Notice {
  id: number
  severity: Severity
  summary: string
  // lỗi: phần giải thích, hiện khi rê chuột hay focus
  detail?: string
  action?: NoticeAction
  undo?: () => void
  retry?: () => void
  // mili giây; 0 là không tự đóng
  life: number
}

export const LIFE = { plain: 4000, timed: 8000 } as const

type Listener = (n: Notice) => void
const listeners = new Set<Listener>()
let seq = 0

export function onNotice(l: Listener): () => void {
  listeners.add(l)
  return () => listeners.delete(l)
}

function emit(n: Omit<Notice, 'id'>): Notice {
  const full = { ...n, id: ++seq }
  listeners.forEach((l) => l(full))
  return full
}

export const notify = {
  success: (summary: string, o: { undo?: () => void; action?: NoticeAction } = {}) =>
    emit({ severity: 'success', summary, ...o, life: o.undo || o.action ? LIFE.timed : LIFE.plain }),
  info: (summary: string, o: { action?: NoticeAction } = {}) =>
    emit({ severity: 'info', summary, ...o, life: o.action ? LIFE.timed : LIFE.plain }),
  error: (summary: string, o: { detail?: string; retry?: () => void } = {}) =>
    emit({ severity: 'error', summary, ...o, life: 0 }),
}

// Ctrl/⌘ Z chạy Undo của thông báo mới nhất còn hiện. NoticeCard đăng ký khi hiện và
// xoá khi đóng
const undos: { id: number; run: () => void }[] = []

export function trackUndo(id: number, run: () => void) {
  undos.push({ id, run })
}

export function forgetUndo(id: number) {
  const i = undos.findIndex((u) => u.id === id)
  if (i >= 0) undos.splice(i, 1)
}

export function latestUndo(): (() => void) | undefined {
  return undos.at(-1)?.run
}

// isUndoShortcut: Ctrl/⌘ Z (không Shift, không Alt) và không đang gõ trong ô nhập
export function isUndoShortcut(e: KeyboardEvent): boolean {
  if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'z') return false
  const t = e.target as { closest?: (s: string) => unknown } | null
  return !t?.closest?.('input, textarea, select, [contenteditable]:not([contenteditable="false"])')
}
```

`frontend/src/lib/countdown.ts`:

```ts
// Đếm ngược tạm dừng được (thông báo có Undo: rê chuột thì dừng); thời gian truyền vào
// để thử được không cần đồng hồ thật
export interface Countdown {
  pause(now: number): void
  resume(now: number): void
  // phần còn lại, 0..1
  left(now: number): number
}

export function countdown(life: number, start: number): Countdown {
  let remain = life
  let since: number | null = start
  return {
    pause(now) {
      if (since === null) return
      remain -= now - since
      since = null
    },
    resume(now) {
      if (since === null) since = now
    },
    left(now) {
      const spent = since === null ? 0 : now - since
      return Math.max(0, remain - spent) / life
    },
  }
}
```

In `features/assets/export/useExport.ts` line 24, change the second argument of `notify.success` from `info.inside ? { label: 'What’s inside', run: info.inside } : undefined` to `info.inside ? { action: { label: 'What’s inside', run: info.inside } } : {}`.

- [ ] **Step 4: Run them to see them pass**

Run: `npx vitest run src/lib/notify.test.ts src/lib/countdown.test.ts`
Expected: PASS (4 tests).

`App.vue` still reads `n.action` from the old shape and compiles, because `Notice` keeps `severity`, `summary` and `action`. Task 3 replaces it.

- [ ] **Step 5: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/lib/notify.ts frontend/src/lib/notify.test.ts frontend/src/lib/countdown.ts frontend/src/lib/countdown.test.ts frontend/src/features/assets/export/useExport.ts
git commit -m "feat(frontend): notices with undo, retry and detail; Ctrl+Z registry

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Filled icons and the toast card

**Files:**
- Create:
  - `frontend/src/components/icons.ts`
  - `frontend/src/components/icons.test.ts`
  - `frontend/src/components/AppIcon.vue`
  - `frontend/src/components/NoticeCard.vue`
- Modify: `frontend/src/App.vue` (rewrite script and template; ConfirmDialog stays as is until Task 5)

**Interfaces:**
- Consumes (from Task 2): `Notice`, `onNotice`, `trackUndo`, `forgetUndo`, `latestUndo`, `isUndoShortcut`, `countdown`.
- Produces:
  - `type IconName = 'tag' | 'trash' | 'mail' | 'logout' | 'alert' | 'tick' | 'error' | 'info' | 'user-plus' | 'sitemap' | 'shield' | 'file' | 'sliders' | 'box'`;
  - `ICONS: Record<IconName, { paths: string[]; evenodd?: boolean }>`;
  - `<AppIcon name>`, which is sized by `font-size` (1em square) and uses `currentColor`.

- [ ] **Step 1: Write the failing test** — `frontend/src/components/icons.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { ICONS } from './icons'

describe('icons', () => {
  it('has every icon the spec names, each a drawable path', () => {
    for (const name of ['tag', 'trash', 'mail', 'logout', 'alert', 'tick', 'user-plus', 'sitemap', 'shield', 'file', 'sliders', 'box'] as const) {
      expect(ICONS[name].paths.length).toBeGreaterThan(0)
      for (const d of ICONS[name].paths) expect(d).toMatch(/^M/)
    }
  })
})
```

- [ ] **Step 2: Run it to see it fail**

Run: `npx vitest run src/components/icons.test.ts`
Expected: FAIL, because `./icons` can't be resolved.

- [ ] **Step 3: Implement icons**

`frontend/src/components/icons.ts`:

```ts
// Biểu tượng đặc (24×24) cho đầu hộp thoại, bong bóng xác nhận và thông báo. Hành động
// trên dòng và menu vẫn dùng PrimeIcons. evenodd: phần khoét (dấu tick, chấm than) thủng
export type IconName =
  | 'tag'
  | 'trash'
  | 'mail'
  | 'logout'
  | 'alert'
  | 'tick'
  | 'error'
  | 'info'
  | 'user-plus'
  | 'sitemap'
  | 'shield'
  | 'file'
  | 'sliders'
  | 'box'

export const ICONS: Record<IconName, { paths: string[]; evenodd?: boolean }> = {
  tag: {
    paths: ['M2.5 4A1.5 1.5 0 0 1 4 2.5h8.2a1.5 1.5 0 0 1 1.06.44l8.3 8.3a1.5 1.5 0 0 1 0 2.12l-8.2 8.2a1.5 1.5 0 0 1-2.12 0l-8.3-8.3A1.5 1.5 0 0 1 2.5 12.2Zm5 5.5a2 2 0 1 0 0-4 2 2 0 0 0 0 4Z'],
    evenodd: true,
  },
  trash: {
    paths: ['M9 2.5h6a1 1 0 0 1 1 1V5h4a1 1 0 1 1 0 2h-1.1l-1 13.1a2 2 0 0 1-2 1.9H8.1a2 2 0 0 1-2-1.9L5.1 7H4a1 1 0 1 1 0-2h4V3.5a1 1 0 0 1 1-1Zm1 2.5h4v-.5h-4Zm-.5 5a.9.9 0 0 0-.9.95l.4 7a.9.9 0 0 0 1.8-.1l-.4-7a.9.9 0 0 0-.9-.85Zm5 0a.9.9 0 0 0-.9.85l-.4 7a.9.9 0 0 0 1.8.1l.4-7a.9.9 0 0 0-.9-.95Z'],
    evenodd: true,
  },
  mail: {
    paths: ['M4 4.5h16a2 2 0 0 1 2 2v.3l-10 6.4L2 6.8v-.3a2 2 0 0 1 2-2Z', 'M2 9.2l9.46 6.05a1 1 0 0 0 1.08 0L22 9.2v8.3a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2Z'],
  },
  logout: {
    paths: [
      'M5 3h8a2 2 0 0 1 2 2v3a1 1 0 1 1-2 0V5H5v14h8v-3a1 1 0 1 1 2 0v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Z',
      'M17.3 7.3a1 1 0 0 1 1.4 0l4 4a1 1 0 0 1 0 1.4l-4 4a1 1 0 1 1-1.4-1.4l2.3-2.3H10a1 1 0 1 1 0-2h9.6l-2.3-2.3a1 1 0 0 1 0-1.4Z',
    ],
  },
  alert: {
    paths: ['M10.3 3.4a2 2 0 0 1 3.4 0l8.2 14.2a2 2 0 0 1-1.7 3H3.8a2 2 0 0 1-1.7-3Zm1.7 5.1a1.1 1.1 0 0 0-1.1 1.15l.2 4.4a.9.9 0 0 0 1.8 0l.2-4.4A1.1 1.1 0 0 0 12 8.5Zm0 8.9a1.2 1.2 0 1 0 0-2.4 1.2 1.2 0 0 0 0 2.4Z'],
    evenodd: true,
  },
  tick: {
    paths: ['M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20Zm4.7 7.7-5.5 5.5a1 1 0 0 1-1.4 0l-2.5-2.5a1 1 0 1 1 1.4-1.4l1.8 1.8 4.8-4.8a1 1 0 1 1 1.4 1.4Z'],
    evenodd: true,
  },
  error: {
    paths: ['M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20Zm0 5a1.1 1.1 0 0 1 1.1 1.15l-.2 5a.9.9 0 0 1-1.8 0l-.2-5A1.1 1.1 0 0 1 12 7Zm0 10.6a1.2 1.2 0 1 1 0-2.4 1.2 1.2 0 0 1 0 2.4Z'],
    evenodd: true,
  },
  info: {
    paths: ['M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20Zm0 5.2a1.2 1.2 0 1 1 0 2.4 1.2 1.2 0 0 1 0-2.4Zm1 4.3v5a1 1 0 1 1-2 0v-5a1 1 0 1 1 2 0Z'],
    evenodd: true,
  },
  'user-plus': {
    paths: ['M9 3a4 4 0 1 1 0 8 4 4 0 0 1 0-8Zm-6 15c0-3 2.7-5 6-5s6 2 6 5v1a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Zm16-10a1 1 0 0 1 1 1v2h2a1 1 0 1 1 0 2h-2v2a1 1 0 1 1-2 0v-2h-2a1 1 0 1 1 0-2h2V9a1 1 0 0 1 1-1Z'],
  },
  sitemap: {
    paths: ['M9 2h6a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-2v3h6a1 1 0 0 1 1 1v3h1a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1h-4a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1h1v-2H6v2h1a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1h1v-3a1 1 0 0 1 1-1h6V8H9a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1Z'],
  },
  shield: {
    paths: ['M11.6 2.1a1 1 0 0 1 .8 0l7 3A1 1 0 0 1 20 6v5c0 5-3.4 9.2-7.7 10.9a1 1 0 0 1-.6 0C7.4 20.2 4 16 4 11V6a1 1 0 0 1 .6-.9Z'],
  },
  file: {
    paths: ['M6 2h7.6a1 1 0 0 1 .7.3l5.4 5.4a1 1 0 0 1 .3.7V20a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2Zm7 1.5V8a1 1 0 0 0 1 1h4.5Z'],
    evenodd: true,
  },
  sliders: {
    paths: [
      'M4 5h9.2a3 3 0 0 1 5.6 0H20a1 1 0 1 1 0 2h-1.2a3 3 0 0 1-5.6 0H4a1 1 0 0 1 0-2Zm0 6h1.2a3 3 0 0 1 5.6 0H20a1 1 0 1 1 0 2h-9.2a3 3 0 0 1-5.6 0H4a1 1 0 1 1 0-2Zm0 6h7.2a3 3 0 0 1 5.6 0H20a1 1 0 1 1 0 2h-3.2a3 3 0 0 1-5.6 0H4a1 1 0 1 1 0-2Z',
    ],
  },
  box: {
    paths: ['M11.5 2.1a1 1 0 0 1 1 0l8 4.5a1 1 0 0 1 .5.9v9a1 1 0 0 1-.5.9l-8 4.5a1 1 0 0 1-1 0l-8-4.5a1 1 0 0 1-.5-.9v-9a1 1 0 0 1 .5-.9ZM5.2 7.6 12 11.4l6.8-3.8L12 3.8Z'],
    evenodd: true,
  },
}
```

`frontend/src/components/AppIcon.vue`:

```vue
<script setup lang="ts">
import { ICONS, type IconName } from './icons'

// Biểu tượng đặc; cỡ theo font-size (1em), màu theo currentColor
defineProps<{ name: IconName }>()
</script>

<template>
  <svg class="app-icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
    <path v-for="(d, i) in ICONS[name].paths" :key="i" :d="d" :fill-rule="ICONS[name].evenodd ? 'evenodd' : undefined" />
  </svg>
</template>

<style scoped>
.app-icon {
  display: block;
  width: 1em;
  height: 1em;
  flex: none;
}
</style>
```

- [ ] **Step 4: Run the test to see it pass**

Run: `npx vitest run src/components/icons.test.ts`
Expected: PASS.

- [ ] **Step 5: The toast card**

`frontend/src/components/NoticeCard.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import { onMounted, onUnmounted, ref } from 'vue'
import { countdown } from '@/lib/countdown'
import { forgetUndo, type Notice, trackUndo } from '@/lib/notify'
import AppIcon from './AppIcon.vue'
import type { IconName } from './icons'

// Một thông báo: biểu tượng tròn đặc, câu, nút (Undo / Retry / nút riêng), ✕. Có Undo
// thì viền là bộ đếm: vệt brand chạy quanh thẻ, ngắn dần; rê chuột hay focus thì dừng.
// Lỗi: một dòng tóm tắt, phần giải thích hiện khi rê chuột hay focus (màn cảm ứng luôn hiện)
const props = defineProps<{ notice: Notice }>()
const emit = defineEmits<{ close: [] }>()

const ICON: Record<Notice['severity'], IconName> = { success: 'tick', info: 'info', warn: 'alert', error: 'error' }

const left = ref(1)
let raf = 0
const timer = props.notice.life ? countdown(props.notice.life, performance.now()) : null
function tick(now: number) {
  if (!timer) return
  left.value = timer.left(now)
  if (left.value <= 0) emit('close')
  else raf = requestAnimationFrame(tick)
}
const pause = () => timer?.pause(performance.now())
const resume = () => timer?.resume(performance.now())

function undo() {
  emit('close')
  props.notice.undo?.()
}
function retry() {
  emit('close')
  props.notice.retry?.()
}
function act() {
  emit('close')
  props.notice.action?.run()
}

onMounted(() => {
  if (timer) raf = requestAnimationFrame(tick)
  if (props.notice.undo) trackUndo(props.notice.id, undo)
})
onUnmounted(() => {
  cancelAnimationFrame(raf)
  forgetUndo(props.notice.id)
})
</script>

<template>
  <div
    :class="['notice', notice.severity, { timed: !!notice.undo }]"
    :role="notice.severity === 'error' ? 'alert' : 'status'"
    :tabindex="notice.detail ? 0 : undefined"
    @mouseenter="pause"
    @mouseleave="resume"
    @focusin="pause"
    @focusout="resume"
  >
    <svg v-if="notice.undo" class="edge" aria-hidden="true">
      <rect class="track" x="0" y="0" width="100%" height="100%" rx="12" />
      <rect class="left" x="0" y="0" width="100%" height="100%" rx="12" pathLength="100" :stroke-dasharray="`${left * 100} 100`" />
    </svg>
    <span class="icon"><AppIcon :name="ICON[notice.severity]" /></span>
    <span class="text">
      <span class="summary">{{ notice.summary }}</span>
      <template v-if="notice.detail">
        <span class="more">Hover for details</span>
        <span class="detail">{{ notice.detail }}</span>
      </template>
    </span>
    <Button v-if="notice.undo" label="Undo" size="small" class="act undo" @click="undo" />
    <Button v-if="notice.retry" label="Retry" size="small" class="act retry" @click="retry" />
    <Button v-if="notice.action" :label="notice.action.label" size="small" text class="act-text" @click="act" />
    <Button icon="pi pi-times" text rounded size="small" severity="secondary" aria-label="Dismiss" @click="emit('close')" />
  </div>
</template>

<style scoped>
.notice {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.7rem;
  min-width: 19rem;
  max-width: 27rem;
  padding: 0.6rem 0.6rem 0.6rem 0.75rem;
  border: 1px solid var(--app-line);
  border-radius: 12px;
  background: var(--p-content-background);
  color: var(--p-text-color);
  box-shadow:
    0 12px 32px rgb(15 23 42 / 0.16),
    0 2px 6px rgb(15 23 42 / 0.08);
  font-size: 0.88rem;
}
/* có Undo: bộ đếm vẽ viền thay cho viền thường */
.notice.timed {
  border-color: transparent;
}
.edge {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}
.edge rect {
  fill: none;
  stroke-width: 2;
}
.edge .track {
  stroke: var(--app-line);
}
.edge .left {
  stroke: var(--app-brand);
}
.icon {
  font-size: 1.6rem;
  color: var(--app-brand-strong);
}
.info .icon {
  color: var(--app-info-strong);
}
.warn .icon {
  color: var(--app-warn-strong);
}
.error .icon {
  color: var(--app-danger-strong);
}
.text {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  line-height: 1.4;
}
.error .summary {
  font-weight: 600;
}
.more {
  color: var(--p-text-muted-color);
  font-size: 0.78rem;
}
.detail {
  max-height: 0;
  overflow: hidden;
  opacity: 0;
  color: var(--p-text-muted-color);
  font-size: 0.82rem;
  transition:
    max-height 0.2s ease,
    opacity 0.2s ease;
}
.notice:hover .detail,
.notice:focus-within .detail,
.notice:focus .detail {
  max-height: 6rem;
  opacity: 1;
}
.notice:hover .more,
.notice:focus-within .more,
.notice:focus .more {
  display: none;
}
@media (hover: none) {
  .detail {
    max-height: none;
    opacity: 1;
  }
  .more {
    display: none;
  }
}
.act {
  flex: none;
  font-weight: 700;
  color: var(--app-on-color);
}
.act.undo {
  background: var(--app-brand-strong);
  border-color: var(--app-brand-strong);
}
.act.retry {
  background: var(--app-danger-strong);
  border-color: var(--app-danger-strong);
}
.act-text {
  flex: none;
  font-weight: 600;
}
</style>
```

- [ ] **Step 6: `App.vue` renders the cards**

Replace the `<script setup>`, the `<template>` and the `<style>` of `frontend/src/App.vue` with the following. The `<ConfirmDialog />` stays as it is until Task 5.

```vue
<script setup lang="ts">
import ConfirmDialog from 'primevue/confirmdialog'
import Toast, { type ToastMessageOptions } from 'primevue/toast'
import { useToast } from 'primevue/usetoast'
import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import NoticeCard from '@/components/NoticeCard.vue'
import { useSession } from '@/lib/auth/session'
import { isUndoShortcut, latestUndo, type Notice, onNotice } from '@/lib/notify'
import { usePreferences } from '@/lib/preferences'

// Thông báo: thẻ riêng (NoticeCard) trong Toast của PrimeVue; tối đa 3, mới nhất ở dưới.
// Toast không tự đóng (không có life): NoticeCard tự đếm giờ rồi gọi closeCallback
const MAX_NOTICES = 3
type NoticeMessage = ToastMessageOptions & { notice: Notice }
const toast = useToast()
const shown: NoticeMessage[] = []
const off = onNotice((n) => {
  const m: NoticeMessage = { severity: n.severity, notice: n }
  shown.push(m)
  while (shown.length > MAX_NOTICES) toast.remove(shown.shift()!)
  toast.add(m)
})
// Toast gắn id vào message khi thêm (kiểu không khai báo id); so theo id vì Toast trả lại
// bản proxy
const idOf = (m: ToastMessageOptions) => (m as { id?: unknown }).id
function forget(e: { message: ToastMessageOptions }) {
  const i = shown.findIndex((m) => idOf(m) === idOf(e.message))
  if (i >= 0) shown.splice(i, 1)
}
// Ctrl/⌘ Z: Undo của thông báo mới nhất còn hiện (không khi đang gõ)
function onKey(e: KeyboardEvent) {
  if (!isUndoShortcut(e)) return
  const undo = latestUndo()
  if (!undo) return
  e.preventDefault()
  undo()
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => {
  off()
  window.removeEventListener('keydown', onKey)
})

// Phiên hết hạn (refresh hỏng hoặc tab khác đăng xuất): về trang đăng nhập
const router = useRouter()
// nạp tuỳ chọn ngay từ đầu để theme áp cả trang đăng nhập
usePreferences()
useSession().setOnExpired(() => {
  const current = router.currentRoute.value
  if (!current.matched.some((r) => r.meta.public)) {
    router.push({ name: 'login', query: { redirect: current.fullPath } })
  }
})
</script>

<template>
  <RouterView />
  <Toast position="bottom-right" :pt="{ root: { class: 'app-notices' } }" @close="forget">
    <template #container="{ message, closeCallback }">
      <NoticeCard :notice="(message as NoticeMessage).notice" @close="closeCallback" />
    </template>
  </Toast>
  <ConfirmDialog />
</template>

<style>
/* Toast của PrimeVue chỉ giữ chỗ và vị trí; thẻ do NoticeCard vẽ */
.app-notices {
  width: auto;
  max-width: calc(100vw - 32px);
}
.app-notices .p-toast-message {
  background: none;
  border: 0;
  box-shadow: none;
  backdrop-filter: none;
  margin: 0 0 0.5rem;
}
</style>
```

- [ ] **Step 7: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/components/icons.ts frontend/src/components/icons.test.ts frontend/src/components/AppIcon.vue frontend/src/components/NoticeCard.vue frontend/src/App.vue
git commit -m "feat(frontend): filled icons and toast card with undo timer, retry and hover detail

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `runAction` and `announce`

**Files:**
- Create:
  - `frontend/src/lib/actions.ts`
  - `frontend/src/lib/actions.test.ts`

**Interfaces:**
- Consumes: `notify`, `onNotice`, `Notice` (Task 2); `describeError` (`lib/errors.ts`).
- Produces:
  - `interface UndoOptions<T> { done: string | ((result: T) => string); undo?: (result: T) => Promise<unknown>; undone?: string; undoFailed?: string }`
  - `interface ActionOptions<T> extends UndoOptions<T> { run: () => Promise<T>; failed?: string }`
  - `announce<T>(result: T, o: UndoOptions<T>): void` reports a success that something else already ran, for example a form that shows its own errors.
  - `runAction<T>(o: ActionOptions<T>): Promise<boolean>`. It returns `false` when `run` failed, and the error is already shown.
- The undo of one announcement runs at most once.

- [ ] **Step 1: Write the failing test** — `frontend/src/lib/actions.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { announce, runAction } from './actions'
import { ApiError } from './errors'
import { type Notice, onNotice } from './notify'

const flush = () => new Promise((r) => setTimeout(r))
let seen: Notice[] = []
let off = onNotice((n) => seen.push(n))
afterEach(() => {
  off()
  seen = []
  off = onNotice((n) => seen.push(n))
})

describe('runAction', () => {
  it('runs, reports done with Undo, and Undo reverses with the run result', async () => {
    const undo = vi.fn().mockResolvedValue(undefined)
    const ok = await runAction({ run: async () => ({ version: 4 }), done: 'Laptop archived.', undo, undone: 'Laptop restored.' })
    expect(ok).toBe(true)
    expect(seen[0]).toMatchObject({ severity: 'success', summary: 'Laptop archived.' })
    seen[0].undo!()
    await flush()
    expect(undo).toHaveBeenCalledWith({ version: 4 })
    expect(seen[1]).toMatchObject({ severity: 'success', summary: 'Laptop restored.' })
  })

  it('has no Undo button without undo, and done may read the result', async () => {
    await runAction({ run: async () => 3, done: (n) => `Ended ${n} sessions.` })
    expect(seen[0].summary).toBe('Ended 3 sessions.')
    expect(seen[0].undo).toBeUndefined()
  })

  it('says what is true now when Undo fails', async () => {
    const err = new ApiError({ type: '/errors/status-changed', title: 'Changed', status: 409, detail: 'Someone else changed it.' })
    await runAction({
      run: async () => null,
      done: 'Laptop archived.',
      undo: () => Promise.reject(err),
      undoFailed: "Couldn't restore Laptop. It is still archived.",
    })
    seen[0].undo!()
    await flush()
    expect(seen[1]).toMatchObject({ severity: 'error', summary: "Couldn't restore Laptop. It is still archived.", detail: 'Someone else changed it.' })
  })

  it('reports a failed run with Retry and returns false', async () => {
    const run = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce(1)
    const ok = await runAction({ run, done: 'Saved.', failed: "Couldn't save." })
    expect(ok).toBe(false)
    expect(seen).toHaveLength(1)
    expect(seen[0]).toMatchObject({ severity: 'error', summary: "Couldn't save.", detail: 'boom' })
    seen[0].retry!()
    await flush()
    expect(run).toHaveBeenCalledTimes(2)
    expect(seen[1]).toMatchObject({ severity: 'success', summary: 'Saved.' })
  })
})

describe('announce', () => {
  it('runs the undo once even if triggered twice', async () => {
    const undo = vi.fn().mockResolvedValue(undefined)
    announce('r', { done: 'Saved.', undo })
    seen[0].undo!()
    seen[0].undo!()
    await flush()
    expect(undo).toHaveBeenCalledOnce()
  })
})
```

- [ ] **Step 2: Run it to see it fail**

Run: `npx vitest run src/lib/actions.test.ts`
Expected: FAIL, because `./actions` can't be resolved.

- [ ] **Step 3: Implement** — `frontend/src/lib/actions.ts`:

```ts
// Hành động thay đổi dữ liệu theo spec "Act now, offer Undo": chạy ngay, báo kết quả, cho
// Undo trong 8 giây. Mutation dùng qua đây phải có meta toast: false (lỗi do đây báo)
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'

export interface UndoOptions<T> {
  // "Laptop archived."; hàm khi câu phụ thuộc kết quả
  done: string | ((result: T) => string)
  // đảo lại, nhận kết quả của lần chạy (vd version mới); không có thì không có nút Undo
  undo?: (result: T) => Promise<unknown>
  // "Laptop restored."
  undone?: string
  // nói điều đang đúng: "Couldn't restore Laptop. It is still archived."
  undoFailed?: string
}

export interface ActionOptions<T> extends UndoOptions<T> {
  run: () => Promise<T>
  // "Couldn't archive Laptop."; không có thì dùng câu của lỗi
  failed?: string
}

// announce: báo thành công (kèm Undo) cho việc đã chạy xong ở chỗ khác (form tự báo lỗi)
export function announce<T>(result: T, o: UndoOptions<T>): void {
  const done = typeof o.done === 'function' ? o.done(result) : o.done
  const undo = o.undo
  if (!undo) {
    notify.success(done)
    return
  }
  let used = false
  notify.success(done, {
    undo: () => {
      // bấm hai lần (nút và Ctrl+Z) chỉ đảo một lần
      if (used) return
      used = true
      undo(result).then(
        () => notify.success(o.undone ?? 'Undone.'),
        (err) => notify.error(o.undoFailed ?? "Couldn't undo that.", { detail: describeError(err) }),
      )
    },
  })
}

export async function runAction<T>(o: ActionOptions<T>): Promise<boolean> {
  let result: T
  try {
    result = await o.run()
  } catch (err) {
    const why = describeError(err)
    notify.error(o.failed ?? why, { detail: o.failed ? why : undefined, retry: () => void runAction(o) })
    return false
  }
  announce(result, o)
  return true
}
```

- [ ] **Step 4: Run it to see it pass**

Run: `npx vitest run src/lib/actions.test.ts`
Expected: PASS (5 tests).

- [ ] **Step 5: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/lib/actions.ts frontend/src/lib/actions.test.ts
git commit -m "feat(frontend): runAction and announce: act now, report, offer Undo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Confirmations, form dialogs, `useDirty`

**Files:**
- Create:
  - `frontend/src/lib/confirm.ts`
  - `frontend/src/lib/confirm.test.ts`
  - `frontend/src/components/FormDialog.vue`
- Modify:
  - `frontend/src/lib/forms.ts` (add `useDirty`)
  - `frontend/src/App.vue` (ConfirmDialog template, `bindConfirm`)
- Test: `frontend/src/lib/forms.test.ts` (new)

**Interfaces:**
- Consumes: `AppIcon`, `IconName` (Task 3).
- Produces, from `lib/confirm.ts`:
  - `interface ConfirmOptions { title: string; body: string; impact?: string[]; action: string; danger: boolean; icon: IconName }`;
  - `bindConfirm(require: (o: ConfirmationOptions) => void): void`;
  - `confirmAction(o): Promise<boolean>`;
  - `confirmDiscard(): Promise<boolean>`;
  - `mayClose(dirty: boolean, ask?: () => Promise<boolean>): Promise<boolean>`.
- Produces `<FormDialog>`:
  - **props:** `title`, `icon: IconName`, `size?: 's'|'m'|'l'` (default `'m'`), `width?: string`, `action?: string`, `busy?`, `error?: string | null`, `dirty?`, `danger?`, `disabled?`, `flush?` (no body padding);
  - **model:** `visible`;
  - **emits:** `submit`;
  - **slots:** default, `hint`, `footer`, `header-extra`.
- Produces, from `lib/forms.ts`: `useDirty(state: () => unknown): { dirty: ComputedRef<boolean>; reset(): void }`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/confirm.test.ts`:

```ts
import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import { describe, expect, it, vi } from 'vitest'
import { bindConfirm, confirmAction, mayClose } from './confirm'

function fake() {
  let last: ConfirmationOptions | undefined
  bindConfirm((o) => (last = o))
  return () => last!
}

const opts = { title: 'Sign Hùng out everywhere?', body: 'Every device is signed out.', action: 'Sign out everywhere', danger: false, icon: 'logout' } as const

describe('confirmAction', () => {
  it('resolves true on accept and passes the view to the template', async () => {
    const last = fake()
    const p = confirmAction({ ...opts, impact: ['3 devices'] })
    expect((last() as unknown as { view: typeof opts }).view.action).toBe('Sign out everywhere')
    last().accept!()
    await expect(p).resolves.toBe(true)
  })
  it('resolves false on reject or hide, and only once', async () => {
    const last = fake()
    const p = confirmAction(opts)
    last().onHide!()
    last().accept!()
    await expect(p).resolves.toBe(false)
    const q = confirmAction(opts)
    last().reject!()
    await expect(q).resolves.toBe(false)
  })
})

describe('mayClose', () => {
  it('closes clean forms without asking, asks for dirty ones', async () => {
    const ask = vi.fn().mockResolvedValue(false)
    await expect(mayClose(false, ask)).resolves.toBe(true)
    expect(ask).not.toHaveBeenCalled()
    await expect(mayClose(true, ask)).resolves.toBe(false)
    ask.mockResolvedValue(true)
    await expect(mayClose(true, ask)).resolves.toBe(true)
  })
})
```

`frontend/src/lib/forms.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import { useDirty } from './forms'

describe('useDirty', () => {
  it('compares the form with its state at the last reset', () => {
    const name = ref('a')
    const form = useDirty(() => ({ name: name.value }))
    form.reset()
    expect(form.dirty.value).toBe(false)
    name.value = 'b'
    expect(form.dirty.value).toBe(true)
    name.value = 'a'
    expect(form.dirty.value).toBe(false)
    name.value = 'c'
    form.reset()
    expect(form.dirty.value).toBe(false)
  })
})
```

- [ ] **Step 2: Run them to see them fail**

Run: `npx vitest run src/lib/confirm.test.ts src/lib/forms.test.ts`
Expected: FAIL. `./confirm` can't be resolved, and `useDirty` isn't exported.

- [ ] **Step 3: Implement `confirm.ts` and `useDirty`**

`frontend/src/lib/confirm.ts`:

```ts
// Hộp xác nhận cho việc không hoàn tác được (đăng xuất mọi nơi, gửi email, bỏ thay đổi).
// Vẽ bằng template riêng của ConfirmDialog trong App.vue: bong bóng biểu tượng, tiêu đề màu
import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import type { IconName } from '@/components/icons'

export interface ConfirmOptions {
  title: string
  body: string
  // các dòng hậu quả, trên nền xám ("3 devices are signed in")
  impact?: string[]
  // nhãn nút hành động: động từ + đối tượng
  action: string
  // đỏ: bỏ thay đổi; xanh dương: không hoàn tác được nhưng không phá gì
  danger: boolean
  icon: IconName
}

type Require = (o: ConfirmationOptions) => void
let require: Require | null = null

// App.vue gắn useConfirm().require lúc khởi động
export function bindConfirm(r: Require) {
  require = r
}

export function confirmAction(o: ConfirmOptions): Promise<boolean> {
  return new Promise((resolve) => {
    if (!require) throw new Error('confirmAction: ConfirmDialog is not mounted')
    let settled = false
    const done = (v: boolean) => {
      if (settled) return
      settled = true
      resolve(v)
    }
    require({
      header: o.title,
      message: o.body,
      // template trong App.vue đọc view
      view: o,
      accept: () => done(true),
      reject: () => done(false),
      onHide: () => done(false),
    } as ConfirmationOptions)
  })
}

export function confirmDiscard(): Promise<boolean> {
  return confirmAction({
    title: 'Discard changes?',
    body: 'What you typed in this form is lost.',
    action: 'Discard',
    danger: true,
    icon: 'alert',
  })
}

// mayClose: form không đổi gì thì đóng luôn; còn thay đổi thì hỏi
export async function mayClose(dirty: boolean, ask: () => Promise<boolean> = confirmDiscard): Promise<boolean> {
  return !dirty || ask()
}
```

Append to `frontend/src/lib/forms.ts`, and change its first import line to `import { computed, ref } from 'vue'`:

```ts
// useDirty: form có thay đổi so với lúc reset() (gọi khi mở hộp thoại, sau khi điền sẵn)
export function useDirty(state: () => unknown) {
  const base = ref(JSON.stringify(state()))
  const dirty = computed(() => JSON.stringify(state()) !== base.value)
  function reset() {
    base.value = JSON.stringify(state())
  }
  return { dirty, reset }
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `npx vitest run src/lib/confirm.test.ts src/lib/forms.test.ts`
Expected: PASS (3 tests).

- [ ] **Step 5: ConfirmDialog template in `App.vue`**

In `frontend/src/App.vue`'s script, add these imports:

```ts
import Button from 'primevue/button'
import { useConfirm } from 'primevue/useconfirm'
import AppIcon from '@/components/AppIcon.vue'
import { bindConfirm, type ConfirmOptions } from '@/lib/confirm'
```

Below the notices block, add:

```ts
// Hộp xác nhận: confirmAction (lib/confirm.ts) gửi tới ConfirmDialog qua đây
bindConfirm(useConfirm().require)
const view = (m: unknown) => (m as { view: ConfirmOptions }).view
```

Replace `<ConfirmDialog />` with:

```vue
  <ConfirmDialog :draggable="false" :pt="{ root: { class: 'app-confirm' } }">
    <template #container="{ message, acceptCallback, rejectCallback }">
      <div :class="['confirm', view(message).danger ? 'danger' : 'info']">
        <span class="bubble"><AppIcon :name="view(message).icon" /></span>
        <h2 class="confirm-title">{{ view(message).title }}</h2>
        <p class="confirm-body">{{ view(message).body }}</p>
        <ul v-if="view(message).impact?.length" class="confirm-impact">
          <li v-for="line in view(message).impact" :key="line">{{ line }}</li>
        </ul>
        <div class="confirm-actions">
          <Button label="Cancel" severity="secondary" outlined :autofocus="view(message).danger" @click="rejectCallback" />
          <Button :label="view(message).action" class="go" :autofocus="!view(message).danger" @click="acceptCallback" />
        </div>
      </div>
    </template>
  </ConfirmDialog>
```

Append to the global `<style>` of `App.vue`:

```css
/* Hộp xác nhận: căn giữa; bong bóng tròn (biểu tượng đặc trắng) nhô nửa lên mép trên, có
   vòng sáng cùng màu; tiêu đề và nút hành động theo loại (đỏ: bỏ; xanh dương: không hoàn
   tác được) */
.app-confirm.p-dialog {
  width: min(26rem, calc(100vw - 2rem));
  margin-top: 2.4rem;
  overflow: visible;
}
.confirm {
  --tone: var(--app-info);
  --tone-strong: var(--app-info-strong);
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 3.2rem 1.4rem 1.3rem;
  text-align: center;
}
.confirm.danger {
  --tone: var(--app-danger);
  --tone-strong: var(--app-danger-strong);
}
.confirm .bubble {
  position: absolute;
  top: -2.4rem;
  left: 50%;
  transform: translateX(-50%);
  display: grid;
  place-items: center;
  width: 4.8rem;
  height: 4.8rem;
  border-radius: 50%;
  background: var(--tone);
  box-shadow:
    0 0 0 5px color-mix(in srgb, var(--tone) 40%, #ffffff),
    0 8px 20px color-mix(in srgb, var(--tone) 35%, transparent);
  color: var(--app-on-color);
  font-size: 2.2rem;
}
.confirm-title {
  font-size: 1.2rem;
  color: var(--tone-strong);
}
.confirm-body {
  margin: 0;
}
.confirm-impact {
  align-self: stretch;
  margin: 0;
  padding: 0.6rem 0.75rem;
  list-style: none;
  border-radius: 8px;
  background: var(--app-soft);
  font-size: 0.86rem;
}
.confirm-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
  align-self: stretch;
  margin-top: 0.4rem;
}
.confirm .go {
  background: var(--tone-strong);
  border-color: var(--tone-strong);
  color: var(--app-on-color);
}
```

- [ ] **Step 6: `FormDialog.vue`** — `frontend/src/components/FormDialog.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Message from 'primevue/message'
import { computed, useId } from 'vue'
import { mayClose } from '@/lib/confirm'
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
      <h2 class="fd-title">{{ title }}</h2>
      <slot name="header-extra" />
      <Button icon="pi pi-times" rounded severity="secondary" class="fd-close" aria-label="Close" @click="requestClose" />
    </template>
    <component :is="action ? 'form' : 'div'" :id="formId" class="fd-content" @submit.prevent="submit">
      <Message v-if="error" severity="error" :closable="false">{{ error }}</Message>
      <slot />
    </component>
    <template #footer>
      <div class="fd-hint"><slot name="hint" /></div>
      <slot name="footer">
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
}
.form-dialog .fd-icon {
  color: var(--app-brand);
  font-size: 1.8rem;
}
.form-dialog .fd-title {
  flex: 1;
  min-width: 0;
  font-family: var(--app-display);
  font-size: 1.5rem;
  font-weight: 800;
  letter-spacing: -0.02em;
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
```

- [ ] **Step 7: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/lib/confirm.ts frontend/src/lib/confirm.test.ts frontend/src/lib/forms.ts frontend/src/lib/forms.test.ts frontend/src/components/FormDialog.vue frontend/src/App.vue
git commit -m "feat(frontend): confirmAction with icon bubble, FormDialog, useDirty

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Empty lists and loading rows

**Files:**
- Create:
  - `frontend/src/components/EmptyState.vue`
  - `frontend/src/components/TableSkeleton.vue`
- Modify: the `#empty` slots of
  - `features/assets/pages/AssetsPage.vue`
  - `features/accounts/pages/AccountsPage.vue` (check for user edits first, per Global Constraints)
  - `features/asset-types/pages/AssetTypesPage.vue`
  - `features/asset-types/pages/TypeSettingsPage.vue`
  - `features/asset-types/components/OptionsDialog.vue`
  - `features/roles/pages/RoleDetailPage.vue`
  - `features/statuses/pages/StatusesPage.vue`
  - `features/export-profiles/pages/ExportProfilesPage.vue`

**Interfaces:**
- Produces:
  - `<EmptyState icon text action?>`, where `icon` is a PrimeIcons class and the `action` event fires when the button is clicked;
  - `<TableSkeleton :rows="3">`.

- [ ] **Step 1: Components**

`frontend/src/components/EmptyState.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'

// Danh sách trống: biểu tượng, một câu, tuỳ chọn một hành động
defineProps<{ icon: string; text: string; action?: string }>()
defineEmits<{ action: [] }>()
</script>

<template>
  <div class="empty-state">
    <i :class="icon" aria-hidden="true" />
    <p>{{ text }}</p>
    <Button v-if="action" :label="action" size="small" outlined @click="$emit('action')" />
  </div>
</template>

<style scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2rem 1rem;
  text-align: center;
  color: var(--p-text-muted-color);
}
.empty-state i {
  font-size: 1.6rem;
}
.empty-state p {
  margin: 0;
  max-width: 40ch;
}
</style>
```

`frontend/src/components/TableSkeleton.vue`:

```vue
<script setup lang="ts">
import Skeleton from 'primevue/skeleton'

// Dòng giả trong lúc trang đầu đang tải (đặt trong #empty của DataTable)
withDefaults(defineProps<{ rows?: number }>(), { rows: 3 })
</script>

<template>
  <div class="table-skeleton" aria-busy="true" aria-label="Loading">
    <Skeleton v-for="i in rows" :key="i" height="0.9rem" />
  </div>
</template>

<style scoped>
.table-skeleton {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 0.6rem 0;
}
</style>
```

- [ ] **Step 2: Use them in every list**

In each file:
- import both components;
- replace the `<template #empty>…</template>` content with `<TableSkeleton v-if="<loading>" /><EmptyState v-else … />`.

`<loading>` is the `isLoading` of the query that feeds the table. Destructure it from the existing `useX(...)` call, for example `const { data: profiles, isFetching, isLoading } = useExportProfiles(true)`. Where the page holds the query object, use `<query>.isLoading.value`.

| File | Loading source | EmptyState props |
|---|---|---|
| AssetsPage.vue | the asset list query | `icon="pi pi-box"`, text `No assets match these filters.` |
| AccountsPage.vue | the accounts query | `icon="pi pi-users"`, text `No accounts match these filters.` |
| AssetTypesPage.vue | the types query | `icon="pi pi-sitemap"`, text `` state.q ? `No types match "${state.q}".` : 'No asset types here.' `` |
| TypeSettingsPage.vue | `useAssetType` query | `icon="pi pi-tag"`, text `No attributes yet.`, and when `canManage`: `action="Add attribute"` `@action="openAttribute(null)"` |
| OptionsDialog.vue | none (no skeleton) | `icon="pi pi-list"`, text `No options yet.` |
| RoleDetailPage.vue | `membersLoading` | `icon="pi pi-users"`, text `Nobody has this role yet.` |
| StatusesPage.vue | the statuses query | `icon="pi pi-tag"`, text `` `No ${KIND_INFO[k].label.toLowerCase()} statuses.` `` |
| ExportProfilesPage.vue | `isLoading` from `useExportProfiles` | `icon="pi pi-file-export"`, text `No profiles yet. Save one from Export report… on any asset list.` |

- [ ] **Step 3: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/components/EmptyState.vue frontend/src/components/TableSkeleton.vue frontend/src/features/assets/pages/AssetsPage.vue frontend/src/features/accounts/pages/AccountsPage.vue frontend/src/features/asset-types/pages/AssetTypesPage.vue frontend/src/features/asset-types/pages/TypeSettingsPage.vue frontend/src/features/asset-types/components/OptionsDialog.vue frontend/src/features/roles/pages/RoleDetailPage.vue frontend/src/features/statuses/pages/StatusesPage.vue frontend/src/features/export-profiles/pages/ExportProfilesPage.vue
git commit -m "feat(frontend): empty states and loading rows on every list

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Assets — retire, restore, bulk and save with Undo

**Files:**
- Modify:
  - `frontend/src/features/assets/api.ts`
  - `frontend/src/features/assets/bulk.ts`
  - `frontend/src/features/assets/values.ts`
  - `frontend/src/features/assets/useAssetActions.ts`
  - `frontend/src/features/assets/components/RetireDialog.vue`
  - `frontend/src/features/assets/components/BulkActionDialog.vue`
  - `frontend/src/features/assets/components/AttributeFilterPopover.vue`
  - `frontend/src/features/assets/pages/AssetFormPage.vue`
  - `frontend/src/features/assets/pages/AssetsPage.vue`
  - `frontend/src/features/assets/pages/AssetDetailPage.vue`
- Test:
  - `frontend/src/features/assets/bulk.test.ts`
  - `frontend/src/features/assets/values.test.ts`

**Interfaces:**
- Consumes: `runAction`, `announce` (Task 4); `FormDialog`, `useDirty` (Task 5).
- Produces:
  - `retiredItems(r: BulkResult, rows: readonly { id: string; version: number }[]): BulkItemRef[]`;
  - `statusGroups(r: BulkResult, rows: readonly { id: string; version: number; status_id: string }[], target: string): { statusId: string; items: BulkItemRef[] }[]`;
  - `assetBodyOf(a: AssetDetail): AssetBody`;
  - `fetchAsset(id: string): Promise<AssetDetail>`;
  - `useAssetActions().restore(a)`, which replaces `askRestore`.

- [ ] **Step 1: Write the failing tests**

Append to `frontend/src/features/assets/bulk.test.ts`. Add `retiredItems, statusGroups` to its import from `./bulk`.

```ts
describe('bulk undo', () => {
  const rows = [
    { id: 'a', tag: 'A', version: 3, status_id: 's1' },
    { id: 'b', tag: 'B', version: 1, status_id: 's2' },
    { id: 'c', tag: 'C', version: 7, status_id: 's1' },
    { id: 'd', tag: 'D', version: 2, status_id: 'target' },
  ]
  const result = { succeeded: ['a', 'b', 'd'], failed: [{ id: 'c', problem: { type: 't', title: 'Changed', status: 409 } }] }

  it('restores only the assets that were retired, at their new version', () => {
    expect(retiredItems(result, rows)).toEqual([
      { id: 'a', version: 4 },
      { id: 'b', version: 2 },
      { id: 'd', version: 3 },
    ])
  })

  it('groups status undo by previous status and skips assets that already had the target', () => {
    expect(statusGroups(result, rows, 'target')).toEqual([
      { statusId: 's1', items: [{ id: 'a', version: 4 }] },
      { statusId: 's2', items: [{ id: 'b', version: 2 }] },
    ])
  })
})
```

Append to `frontend/src/features/assets/values.test.ts`. Add `assetBodyOf` to its import from `./values`, and `import type { AssetDetail } from '@/lib/api/types'`.

```ts
describe('assetBodyOf', () => {
  it('rebuilds the PUT body from a saved asset', () => {
    const a = {
      id: 'x', tag: 'LAP-1', name: 'Laptop', description: 'Old', version: 5,
      asset_type: { id: 't1' }, status: { id: 's1' },
      location_id: 'l1', purchase_date: '2026-01-02',
      attributes: [
        { key: 'ram', label: 'RAM', data_type: 'number', value: 16 },
        { key: 'note', label: 'Note', data_type: 'text', value: null },
        { key: 'os', label: 'OS', data_type: 'select', value: 'opt-1', option_label: 'Windows' },
      ],
    } as unknown as AssetDetail
    expect(assetBodyOf(a)).toEqual({
      name: 'Laptop', description: 'Old', asset_type_id: 't1', status_id: 's1',
      location_id: 'l1', holder_member_id: undefined, purchase_date: '2026-01-02',
      attributes: { ram: 16, os: 'opt-1' },
    })
  })
})
```

- [ ] **Step 2: Run them to see them fail**

Run: `npx vitest run src/features/assets/bulk.test.ts src/features/assets/values.test.ts`
Expected: FAIL, because `retiredItems`, `statusGroups` and `assetBodyOf` aren't exported.

- [ ] **Step 3: Implement the helpers**

Append to `frontend/src/features/assets/bulk.ts`. Also add `import type { BulkItemRef } from './api'`.

```ts
// Undo của retire hàng loạt: tài sản đã retire, với version mới (mỗi lần ghi tăng 1)
export function retiredItems(r: BulkResult, rows: readonly { id: string; version: number }[]): BulkItemRef[] {
  const ok = new Set(r.succeeded)
  return rows.filter((x) => ok.has(x.id)).map((x) => ({ id: x.id, version: x.version + 1 }))
}

// Undo của đổi status hàng loạt: nhóm theo status cũ để đặt lại từng nhóm. Tài sản vốn
// đã có status đích thì server không ghi (version giữ nguyên) nên không cần đặt lại
export function statusGroups(
  r: BulkResult,
  rows: readonly { id: string; version: number; status_id: string }[],
  target: string,
): { statusId: string; items: BulkItemRef[] }[] {
  const ok = new Set(r.succeeded)
  const groups = new Map<string, BulkItemRef[]>()
  for (const x of rows) {
    if (!ok.has(x.id) || x.status_id === target) continue
    const g = groups.get(x.status_id) ?? []
    g.push({ id: x.id, version: x.version + 1 })
    groups.set(x.status_id, g)
  }
  return [...groups].map(([statusId, items]) => ({ statusId, items }))
}
```

Append to `frontend/src/features/assets/values.ts`. Add `AssetDetail` to the type import from `@/lib/api/types`, and add `import type { AssetBody } from './api'`.

```ts
// assetBodyOf: body PUT dựng lại từ tài sản đã lưu (Undo của "Save"), cùng định dạng form gửi
export function assetBodyOf(a: AssetDetail): AssetBody {
  return {
    name: a.name,
    description: a.description,
    asset_type_id: a.asset_type.id,
    status_id: a.status.id,
    location_id: a.location_id,
    holder_member_id: a.holder_member_id,
    purchase_date: a.purchase_date,
    attributes: toApiValues(a.attributes, fromApiValues(a.attributes, a.attributes)),
  }
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `npx vitest run src/features/assets/bulk.test.ts src/features/assets/values.test.ts`
Expected: PASS.

- [ ] **Step 5: API**

In `frontend/src/features/assets/api.ts`:
- Pass `false` as the second argument of `useAssetMutation` in `useRetireAsset`, `useRestoreAsset`, `useBulkRetire` and `useBulkStatus`.
- Add:

```ts
// fetchAsset: đọc một lần ngoài query (lý do retire cho Undo của restore)
export function fetchAsset(id: string) {
  return unwrap(inventoryApi.GET('/assets/{assetID}', path(id)))
}
```

- [ ] **Step 6: `useAssetActions` — restore runs at once**

Replace `frontend/src/features/assets/useAssetActions.ts` with:

```ts
// Hành động nhanh trên một tài sản, dùng chung cho danh sách và trang tài sản: mở, sửa,
// retire (RetireDialog, có Undo), restore (chạy ngay, có Undo)
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { runAction } from '@/lib/actions'
import { openLocation } from '@/lib/navigation'
import { fetchAsset, useRestoreAsset, useRetireAsset } from './api'

export interface ActionAsset {
  id: string
  tag: string
  name: string
  version: number
  retired_at?: string
  // trang tài sản có sẵn; dòng danh sách không có thì đọc lúc restore
  retired_reason?: string
}

export function useAssetActions() {
  const router = useRouter()
  const restoreAsset = useRestoreAsset()
  const retireAsset = useRetireAsset()

  // retireTarget + retireOpen gắn vào <RetireDialog>
  const retireTarget = ref<ActionAsset | null>(null)
  const retireOpen = ref(false)

  return {
    retireTarget,
    retireOpen,
    open(a: ActionAsset, e?: MouseEvent, newTab?: boolean) {
      openLocation(router, `/assets/${a.id}`, e, newTab)
    },
    edit(a: ActionAsset, e?: MouseEvent) {
      openLocation(router, `/assets/${a.id}/edit`, e)
    },
    askRetire(a: ActionAsset) {
      retireTarget.value = a
      retireOpen.value = true
    },
    restore(a: ActionAsset) {
      return runAction({
        run: async () => {
          // giữ lý do để Undo retire lại đúng như cũ
          const reason = a.retired_reason ?? (await fetchAsset(a.id)).retired_reason ?? ''
          const restored = await restoreAsset.mutateAsync({ id: a.id, version: a.version })
          return { restored, reason }
        },
        done: `${a.tag} restored.`,
        failed: `Couldn't restore ${a.tag}.`,
        undo: ({ restored, reason }) => retireAsset.mutateAsync({ id: a.id, reason, version: restored.version }),
        undone: `${a.tag} retired again.`,
        undoFailed: `Couldn't retire ${a.tag} again. It stays restored.`,
      })
    },
  }
}
```

In `AssetsPage.vue` (context menu and row action) and `AssetDetailPage.vue`, replace every `actions.askRestore(` with `actions.restore(`.

- [ ] **Step 7: Retire is not red**

- `AssetDetailPage.vue`: on the `label="Retire"` Button, change `severity="danger"` to `severity="secondary"`.
- `AssetsPage.vue`:
  - on the bulk `label="Retire"` Button (line ~361), change `severity="danger"` to `severity="secondary"`;
  - remove `severity="danger"` (or the `danger` attribute) from the row Retire `IconAction` (line ~451).

- [ ] **Step 8: `RetireDialog` → FormDialog, with Undo**

Replace `frontend/src/features/assets/components/RetireDialog.vue` with:

```vue
<script setup lang="ts">
import Textarea from 'primevue/textarea'
import { ref, watch } from 'vue'
import FormDialog from '@/components/FormDialog.vue'
import { announce } from '@/lib/actions'
import { useDirty, useFormErrors } from '@/lib/forms'
import { useRestoreAsset, useRetireAsset } from '../api'

// Hộp thoại retire dùng chung cho danh sách và trang tài sản; giữ ô lý do, báo kèm Undo
const props = defineProps<{ asset: { id: string; tag: string; name: string; version: number } | null }>()
const visible = defineModel<boolean>('visible', { required: true })
const emit = defineEmits<{ retired: [] }>()

const reason = ref('')
const retire = useRetireAsset()
const restore = useRestoreAsset()
const errors = useFormErrors()
const form = useDirty(() => reason.value.trim())

watch(visible, (open) => {
  if (!open) return
  reason.value = ''
  errors.clear()
  form.reset()
})

async function submit() {
  const a = props.asset
  if (!a) return
  errors.clear()
  try {
    const retired = await retire.mutateAsync({ id: a.id, reason: reason.value, version: a.version })
    visible.value = false
    emit('retired')
    announce(retired, {
      done: `${a.tag} retired.`,
      undo: (r) => restore.mutateAsync({ id: a.id, version: r.version }),
      undone: `${a.tag} restored.`,
      undoFailed: `Couldn't restore ${a.tag}. It is still retired.`,
    })
  } catch (err) {
    errors.set(err)
  }
}
</script>

<template>
  <FormDialog
    v-model:visible="visible"
    size="s"
    icon="box"
    :title="asset ? `Retire ${asset.tag}` : 'Retire asset'"
    action="Retire"
    :busy="retire.isPending.value"
    :error="errors.general.value"
    :dirty="form.dirty.value"
    @submit="submit"
  >
    <p>{{ asset?.name }} leaves the list unless “Include retired” is on, and can't be edited until restored.</p>
    <div class="field">
      <label for="retire-reason">Reason (optional)</label>
      <Textarea id="retire-reason" v-model="reason" rows="3" />
    </div>
  </FormDialog>
</template>
```

- [ ] **Step 9: `BulkActionDialog` → FormDialog, one Undo for the batch**

In `frontend/src/features/assets/components/BulkActionDialog.vue`:
- Change the `rows` prop type to `{ id: string; tag: string; version: number; status_id: string }[]`.
- Replace the imports:
  - `Dialog` with `FormDialog` (`@/components/FormDialog.vue`);
  - `notify` with `announce` (`@/lib/actions`);
  - add `useDirty, useFormErrors` (`@/lib/forms`), `useRestoreAsset` and `type BulkItemRef` (`../api`), and `retiredItems, statusGroups` (`../bulk`).
- Add `const errors = useFormErrors()`, `const restore = useRestoreAsset()` and `const form = useDirty(() => ({ s: statusId.value, r: reason.value.trim() }))`. In the `watch(visible)` open branch, also call `errors.clear()` and `form.reset()`.
- Replace `run()` with:

```ts
async function restoreEach(items: BulkItemRef[]) {
  const results = await Promise.allSettled(items.map((it) => restore.mutateAsync(it)))
  const failed = results.filter((x) => x.status === 'rejected').length
  if (failed) throw new Error(`${failed} of ${items.length} could not be restored.`)
}
async function setEach(groups: { statusId: string; items: BulkItemRef[] }[]) {
  let failed = 0
  for (const g of groups) failed += (await setStatus.mutateAsync({ items: g.items, statusId: g.statusId })).failed.length
  if (failed) throw new Error(`${failed} could not be changed back.`)
}
const assets = (n: number) => `${n} ${n === 1 ? 'asset' : 'assets'}`

async function run() {
  const rows = [...props.rows]
  const items = rows.map((r) => ({ id: r.id, version: r.version }))
  const retiring = props.mode === 'retire'
  const target = statusId.value!
  errors.clear()
  try {
    const result = retiring
      ? await retire.mutateAsync({ items, reason: reason.value })
      : await setStatus.mutateAsync({ items, statusId: target })
    const s = summarizeBulk(result, rows, retiring ? 'retired' : 'updated')
    emit('done')
    const n = result.succeeded.length
    if (n)
      announce(
        result,
        retiring
          ? {
              done: s.message,
              undo: (r) => restoreEach(retiredItems(r, rows)),
              undone: `${assets(n)} restored.`,
              undoFailed: "Couldn't restore every asset. Some are still retired.",
            }
          : {
              done: s.message,
              undo: (r) => setEach(statusGroups(r, rows, target)),
              undone: `${assets(n)} changed back.`,
              undoFailed: "Couldn't change every asset back.",
            },
      )
    if (s.failures.length) summary.value = s
    else visible.value = false
  } catch (err) {
    errors.set(err)
  }
}
```

- Template: replace `<Dialog …>` with the following, and remove the inner form wrapper and the action buttons:

```vue
<FormDialog
  v-model:visible="visible"
  icon="sliders"
  :title="header"
  :action="summary ? undefined : mode === 'retire' ? 'Retire' : 'Change status'"
  :disabled="mode === 'status' && !statusId"
  :busy="busy"
  :error="errors.general.value"
  :dirty="!summary && form.dirty.value"
  @submit="run"
>
```

The summary list of failures stays in the body as it is.

- In `header`, change `` `Retire ${count.value}?` `` to `` `Retire ${count.value}` ``.

- [ ] **Step 10: Save asset changes offers Undo**

In `AssetFormPage.vue`:
- import `announce` (`@/lib/actions`) and `assetBodyOf` (`../values`);
- in `submit()`, after building `body`, add:

```ts
  // sửa: giữ bản đã lưu để Undo đặt lại
  const before = isEdit.value && asset.value ? { id: asset.value.id, tag: asset.value.tag, body: assetBodyOf(asset.value) } : null
```

and replace `notify.success('Asset saved.')` with:

```ts
    if (before)
      announce(saved, {
        done: `${before.tag} saved.`,
        undo: (s) => replace.mutateAsync({ id: before.id, version: s.version, ...before.body }),
        undone: `Changes to ${before.tag} undone.`,
        undoFailed: `Couldn't undo the changes to ${before.tag}. The saved version stays.`,
      })
    else notify.success(`${saved.tag} created.`)
```

- [ ] **Step 11: Attribute filter popover follows the popover rules**

In `features/assets/components/AttributeFilterPopover.vue`:
- **Lead line:** as the first child of the `<form class="filter-form">`, add:

  ```vue
  <p class="pop-lead">Show only assets whose attribute matches.</p>
  ```

  with this scoped style:

  ```css
  .pop-lead {
    margin: 0;
    font-size: 0.85rem;
    color: var(--p-text-muted-color);
  }
  ```

- **Button order:** swap the two buttons at the end of the form so Cancel comes first, then "Add filter" (spec: footer Cancel, then the action).

Enter already submits through the form, and Esc or clicking outside already closes it (PrimeVue Popover).

- [ ] **Step 12: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/assets
git commit -m "feat(frontend): asset retire, restore, bulk actions and save offer Undo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Asset types — archive, attributes, options and order with Undo

**Files:**
- Modify:
  - `frontend/src/features/asset-types/api.ts`
  - `frontend/src/features/asset-types/pages/TypeSettingsPage.vue`
  - `frontend/src/features/asset-types/components/OptionsDialog.vue`
  - `frontend/src/features/asset-types/components/AttributeDialog.vue`
  - `frontend/src/features/asset-types/pages/AssetTypesPage.vue`

**Interfaces:**
- Consumes: `runAction` (Task 4); `FormDialog`, `useDirty` (Task 5); `POST …/attributes/{attributeID}/restore` and `…/options/{optionID}/restore` (Plan A).
- Produces: `useRestoreAttribute()` with variables `{ typeId, attrId }`; `useRestoreOption()` with variables `{ typeId, attrId, optionId }`.

- [ ] **Step 1: API hooks**

In `features/asset-types/api.ts`:
- Pass `false` as the second argument of `useTypeMutation` in `useArchiveAssetType`, `useRestoreAssetType`, `useRemoveAttribute`, `useRemoveOption`, `useReorderAttributes` and `useReorderOptions`.
- Add:

```ts
// Undo của bỏ thuộc tính / option (xoá mềm)
export function useRestoreAttribute() {
  return useTypeMutation(
    ({ typeId, attrId }: { typeId: string; attrId: string }) =>
      unwrap(inventoryApi.POST('/asset-types/{typeID}/attributes/{attributeID}/restore', attrPath(typeId, attrId))),
    false,
  )
}

export function useRestoreOption() {
  return useTypeMutation(
    ({ typeId, attrId, optionId }: { typeId: string; attrId: string; optionId: string }) =>
      unwrap(
        inventoryApi.POST('/asset-types/{typeID}/attributes/{attributeID}/options/{optionID}/restore', {
          params: { path: { typeID: typeId, attributeID: attrId, optionID: optionId } },
        }),
      ),
    false,
  )
}
```

- [ ] **Step 2: TypeSettingsPage**

Remove the `useConfirm` import and `const confirm = useConfirm()`. Import `runAction` (`@/lib/actions`) and `useRestoreAttribute` (`../api`).

Replace `onReorder` with:

```ts
function onReorder(e: DataTableRowReorderEvent) {
  const activeIds = (list: Attribute[]) => list.filter((a) => !a.removed).map((a) => a.id)
  const before = activeIds(attributes.value)
  attributes.value = e.value as Attribute[]
  const after = activeIds(attributes.value)
  void runAction({
    run: () => reorder.mutateAsync({ typeId: props.typeId, ids: after }),
    done: 'Attributes reordered.',
    failed: "Couldn't save the new order.",
    undo: () => reorder.mutateAsync({ typeId: props.typeId, ids: before }),
    undone: 'Order put back.',
    undoFailed: "Couldn't put the order back.",
  })
}
```

Replace `toggleArchived` with:

```ts
function toggleArchived() {
  const t = type.value
  if (!t) return
  if (t.archived_at)
    return runAction({
      run: () => restore.mutateAsync(t.id),
      done: `${t.name} restored.`,
      failed: `Couldn't restore ${t.name}.`,
      undo: () => archive.mutateAsync(t.id),
      undone: `${t.name} archived again.`,
      undoFailed: `Couldn't archive ${t.name} again. It stays available.`,
    })
  return runAction({
    run: () => archive.mutateAsync(t.id),
    done: `${t.name} archived.`,
    failed: `Couldn't archive ${t.name}.`,
    undo: () => restore.mutateAsync(t.id),
    undone: `${t.name} restored.`,
    undoFailed: `Couldn't restore ${t.name}. It is still archived.`,
  })
}
```

Replace `askRemove` with the following, then replace every `askRemove(` in this file's template and row menu with `removeAttribute(`:

```ts
const removeAttr = useRemoveAttribute()
const restoreAttr = useRestoreAttribute()
function removeAttribute(a: Attribute) {
  const ids = { typeId: props.typeId, attrId: a.id }
  return runAction({
    run: () => removeAttr.mutateAsync(ids),
    done: `${a.label} removed.`,
    failed: `Couldn't remove ${a.label}.`,
    undo: () => restoreAttr.mutateAsync(ids),
    undone: `${a.label} is back.`,
    undoFailed: `Couldn't bring ${a.label} back. It stays removed.`,
  })
}
```

Change the "Archive" button so it isn't red, if it has `severity="danger"`: use `severity="secondary"`.

- [ ] **Step 3: OptionsDialog**

- Remove `useConfirm` and `notify` imports except where `save()` still uses `notify`; keep `notify` for "Option saved.".
- Import `runAction`, `FormDialog` and `useRestoreOption`.
- Rename `const remove = useRemoveOption()` to `const removeOpt = useRemoveOption()`, and add `const restoreOpt = useRestoreOption()`.
- Replace `onReorder` and `askRemove`:

```ts
function onReorder(e: DataTableRowReorderEvent) {
  const before = rows.value.map((o) => o.id)
  rows.value = e.value as Option[]
  const after = rows.value.map((o) => o.id)
  void runAction({
    run: () => reorder.mutateAsync({ ...ids(), ids: after }),
    done: 'Options reordered.',
    failed: "Couldn't save the new order.",
    undo: () => reorder.mutateAsync({ ...ids(), ids: before }),
    undone: 'Order put back.',
    undoFailed: "Couldn't put the order back.",
  })
}

function removeOption(o: Option) {
  const ref = { ...ids(), optionId: o.id }
  return runAction({
    run: () => removeOpt.mutateAsync(ref),
    done: `${o.label} removed.`,
    failed: `Couldn't remove ${o.label}.`,
    undo: () => restoreOpt.mutateAsync(ref),
    undone: `${o.label} is back.`,
    undoFailed: `Couldn't bring ${o.label} back. It stays removed.`,
  })
}
```

- Template:
  - change the remove IconAction's `@click="askRemove(o)"` to `@click="removeOption(o)"`;
  - replace `<Dialog …>`/`</Dialog>` with `<FormDialog v-model:visible="visible" size="l" icon="sliders" :title="`Options of ${attribute?.label ?? ''}`">`/`</FormDialog>`. There's no `action`, so the footer is Close and the body is a `div`, and the add-option `<form>` stays.

- [ ] **Step 4: AttributeDialog and New asset type → FormDialog**

Apply the Dialog migration recipe:
- **AttributeDialog.vue:**
  - `size="m"`, `icon="tag"`;
  - title `isNew ? 'Add attribute' : 'Edit attribute'`, action `isNew ? 'Add attribute' : 'Save attribute'`;
  - dirty over every field ref the form edits (key, label, data type, unit, required, options text).
- **AssetTypesPage.vue**, the `creating` dialog:
  - `size="m"`, `icon="sitemap"`, title `New asset type`, action `Create type`;
  - dirty over code, name and description;
  - call `form.reset()` where the page opens the dialog (sets `creating = true`).

- [ ] **Step 5: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/asset-types
git commit -m "feat(frontend): asset type archive, attribute/option removal and reorder offer Undo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Statuses — archive, restore, default, order with Undo

**Files:**
- Modify:
  - `frontend/src/features/statuses/api.ts`
  - `frontend/src/features/statuses/pages/StatusesPage.vue`

**Interfaces:**
- Consumes: `runAction` (Task 4); `FormDialog`, `useDirty` (Task 5).

- [ ] **Step 1: API**

In `features/statuses/api.ts`:
- pass `false` as the second argument of `useStatusMutation` in `useArchiveStatus` and `useRestoreStatus`;
- in `useReorderStatuses`, change `meta: { toast: true }` to `meta: { toast: false }`.

- [ ] **Step 2: StatusesPage**

Remove `useConfirm`, `const confirm` and the `Dialog` import. Import `runAction` and `FormDialog`.

Replace `saveLane`:

```ts
function saveLane(kind: StatusKind, laneIds: string[]) {
  const all = statuses.value ?? []
  const before = all.filter((s) => !s.archived_at).sort((a, b) => a.position - b.position).map((s) => s.id)
  const after = orderAfterMove(all, kind, laneIds)
  void runAction({
    run: () => reorder.mutateAsync(after),
    done: 'Statuses reordered.',
    failed: "Couldn't save the new order.",
    undo: () => reorder.mutateAsync(before),
    undone: 'Order put back.',
    undoFailed: "Couldn't put the order back.",
  })
}
```

Replace `makeDefault`:

```ts
function makeDefault(s: Status) {
  const prev = (statuses.value ?? []).find((x) => x.kind === s.kind && x.is_default && !x.archived_at)
  const kind = KIND_INFO[s.kind].label.toLowerCase()
  return runAction({
    run: () => update.mutateAsync({ id: s.id, make_default: true }),
    done: `${s.name} is now the default ${kind} status.`,
    failed: `Couldn't make ${s.name} the default.`,
    undo: prev ? () => update.mutateAsync({ id: prev.id, make_default: true }) : undefined,
    undone: prev && `${prev.name} is the default ${kind} status again.`,
    undoFailed: `Couldn't change the default back. ${s.name} is still the default.`,
  })
}
```

Replace `askArchive` and `doRestore`. In the template, rename their callers from `askArchive(` to `archiveStatus(` and from `doRestore(` to `restoreStatus(`.

```ts
const archive = useArchiveStatus()
const restore = useRestoreStatus()
function archiveStatus(s: Status) {
  return runAction({
    run: () => archive.mutateAsync(s.id),
    done: `${s.name} archived.`,
    failed: `Couldn't archive ${s.name}.`,
    undo: () => restore.mutateAsync(s.id),
    undone: `${s.name} restored.`,
    undoFailed: `Couldn't restore ${s.name}. It is still archived.`,
  })
}
function restoreStatus(s: Status) {
  return runAction({
    run: () => restore.mutateAsync(s.id),
    done: `${s.name} restored.`,
    failed: `Couldn't restore ${s.name}.`,
    undo: () => archive.mutateAsync(s.id),
    undone: `${s.name} archived again.`,
    undoFailed: `Couldn't archive ${s.name} again. It stays available.`,
  })
}
```

Make the archive IconAction non-red: remove its `danger` attribute if present.

Edit status dialog: apply the recipe with `size="s"`, `icon="tag"`, title `Edit status`, action `Save status`, and `useDirty(() => editName.value.trim())`. Call `form.reset()` at the end of `openEdit`.

- [ ] **Step 3: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/statuses
git commit -m "feat(frontend): status archive, restore, default and reorder offer Undo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Accounts — disable/enable with Undo, sends confirmed

**Files:**
- Modify:
  - `frontend/src/features/accounts/api.ts`
  - `frontend/src/features/accounts/useAccountActions.ts` (rewrite)
  - `frontend/src/features/accounts/pages/AccountDetailPage.vue`
  - `frontend/src/features/accounts/components/CreateAccountDialog.vue`

**Interfaces:**
- Consumes: `runAction` (Task 4); `confirmAction`, `FormDialog`, `useDirty` (Task 5).
- Produces: `useAccountActions()` keeps the same method names (`isSelf`, `resendInvitation`, `sendReset`, `disable`, `enable`, `signOutEverywhere`). `AccountsPage.vue` needs no change.

- [ ] **Step 1: API**

In `features/accounts/api.ts`, pass `false` as the second argument of `useAccountMutation` in `useAssignRoles`, `useDisableAccount`, `useEnableAccount`, `useResendInvitation`, `useSendPasswordReset` and `useSignOutAccount`.

- [ ] **Step 2: Rewrite `useAccountActions.ts`**

```ts
// Hành động trên một tài khoản, dùng chung cho danh sách và trang tài khoản. Khoá / mở
// khoá chạy ngay và có Undo; gửi email và đăng xuất mọi nơi không hoàn tác được nên hỏi trước
import type { Account } from '@/lib/api/types'
import { runAction } from '@/lib/actions'
import { useSession } from '@/lib/auth/session'
import { confirmAction } from '@/lib/confirm'
import {
  useDisableAccount,
  useEnableAccount,
  useResendInvitation,
  useSendPasswordReset,
  useSignOutAccount,
} from './api'

type Target = Pick<Account, 'id' | 'name' | 'email' | 'status'>

export function useAccountActions() {
  const session = useSession()
  const disable = useDisableAccount()
  const enable = useEnableAccount()
  const resend = useResendInvitation()
  const reset = useSendPasswordReset()
  const signOut = useSignOutAccount()

  const isSelf = (a: Target) => a.id === session.me?.account.id

  return {
    isSelf,
    async resendInvitation(a: Target) {
      const ok = await confirmAction({
        title: `Resend the invitation to ${a.email}?`,
        body: 'A new link is emailed. The previous link stops working.',
        action: 'Send invitation',
        danger: false,
        icon: 'mail',
      })
      if (ok) await runAction({ run: () => resend.mutateAsync(a.id), done: `Invitation sent to ${a.email}.`, failed: `Couldn't send the invitation to ${a.email}.` })
    },
    async sendReset(a: Target) {
      const ok = await confirmAction({
        title: `Send ${a.name} a password reset link?`,
        body: `The link goes to ${a.email} and works for one hour.`,
        action: 'Send link',
        danger: false,
        icon: 'mail',
      })
      if (ok) await runAction({ run: () => reset.mutateAsync(a.id), done: `Reset link sent to ${a.email}.`, failed: `Couldn't send the reset link to ${a.email}.` })
    },
    disable(a: Target) {
      return runAction({
        run: () => disable.mutateAsync(a.id),
        done: `${a.name} disabled.`,
        failed: `Couldn't disable ${a.name}.`,
        undo: () => enable.mutateAsync(a.id),
        undone: `${a.name} enabled again.`,
        undoFailed: `Couldn't enable ${a.name} again. The account is still disabled.`,
      })
    },
    enable(a: Target) {
      return runAction({
        run: () => enable.mutateAsync(a.id),
        done: `${a.name} enabled.`,
        failed: `Couldn't enable ${a.name}.`,
        undo: () => disable.mutateAsync(a.id),
        undone: `${a.name} disabled again.`,
        undoFailed: `Couldn't disable ${a.name} again. The account stays enabled.`,
      })
    },
    async signOutEverywhere(a: Target) {
      const ok = await confirmAction({
        title: `Sign ${a.name} out everywhere?`,
        body: 'Every device they are signed in on is signed out. They need their password to come back.',
        action: 'Sign out everywhere',
        danger: false,
        icon: 'logout',
      })
      if (!ok) return
      await runAction({
        run: () => signOut.mutateAsync(a.id),
        done: (r) => (r.revoked ? `${a.name} signed out on ${r.revoked} device${r.revoked === 1 ? '' : 's'}.` : `${a.name} had no open sessions.`),
        failed: `Couldn't sign ${a.name} out.`,
      })
    },
  }
}
```

The Disable buttons in `AccountsPage.vue` and `AccountDetailPage.vue` must not be red. If either one has `severity="danger"` or a `danger` IconAction attribute, change it to secondary or remove the attribute. AccountsPage may hold user edits, so apply the Global Constraints check before touching it.

- [ ] **Step 3: Account roles save with Undo**

In `AccountDetailPage.vue`, import `runAction`, and replace `saveRoles`:

```ts
async function saveRoles() {
  const before = [...saved.value]
  const ok = await runAction({
    run: () => assign.mutateAsync({ id: props.id, roleIds: current.value }),
    done: `Roles of ${account.value?.name ?? 'the account'} saved.`,
    failed: "Couldn't save the roles.",
    undo: () => assign.mutateAsync({ id: props.id, roleIds: before }),
    undone: 'Roles put back.',
    undoFailed: "Couldn't put the roles back. The new roles stay.",
  })
  if (ok) draft.value = null
}
```

- [ ] **Step 4: Invite account → FormDialog**

Apply the recipe to `CreateAccountDialog.vue`:
- `size="m"`, `icon="user-plus"`, title `Invite account`, action `Send invite`;
- `busy` = `create.isPending.value`;
- `form = useDirty(() => ({ e: email.value.trim(), n: name.value.trim(), r: roleIds.value }))`, with `form.reset()` at the end of the open branch of `watch(visible)`;
- move `<p class="hint">They get an email…</p>` into `<template #hint>` as plain text.

- [ ] **Step 5: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/accounts
git commit -m "feat(frontend): account disable/enable and roles offer Undo; emails and sign-out confirm

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Roles — delete, permissions, people with Undo

**Files:**
- Modify:
  - `frontend/src/features/roles/api.ts`
  - `frontend/src/features/roles/pages/RoleDetailPage.vue`
  - `frontend/src/features/roles/pages/RolesPage.vue`

**Interfaces:**
- Consumes: `runAction` (Task 4); `FormDialog`, `useDirty` (Task 5); `useAssignRoles` (`toast: false` since Task 10); `POST /roles/{roleID}/restore` (Plan A).
- Produces: `useRestoreRole()` with variables `id: string`.

- [ ] **Step 1: API**

In `features/roles/api.ts`:
- pass `false` to `useRoleMutation` in `useSetRolePermissions` and `useDeleteRole`;
- add:

```ts
// Undo của xoá role (xoá mềm)
export function useRestoreRole() {
  return useRoleMutation(
    (id: string) => unwrap(identityApi.POST('/roles/{roleID}/restore', { params: { path: { roleID: id } } })),
    false,
  )
}
```

- [ ] **Step 2: RoleDetailPage actions**

Remove `useConfirm` and `const confirm`. Import `runAction`, `FormDialog`, `useDirty` and `useRestoreRole`.

Replace `savePermissions`:

```ts
async function savePermissions() {
  const before = [...saved.value]
  const ok = await runAction({
    run: () => setPerms.mutateAsync({ id: props.id, permissions: current.value }),
    done: `Permissions of ${role.value?.name ?? 'the role'} saved.`,
    failed: "Couldn't save the permissions.",
    undo: () => setPerms.mutateAsync({ id: props.id, permissions: before }),
    undone: 'Permissions put back.',
    undoFailed: "Couldn't put the permissions back. The new permissions stay.",
  })
  if (ok) draft.value = null
}
```

Replace `askDelete` with `deleteRole`, and rename its template caller:

```ts
const remove = useDeleteRole()
const restoreRole = useRestoreRole()
async function deleteRole() {
  // role còn người giữ thì API trả 409: nói trước
  if (people.value) {
    notify.info(`${role.value?.name} is still held by ${peopleLabel(people.value)}. Remove them from the role first.`)
    return
  }
  const name = role.value?.name ?? 'The role'
  const ok = await runAction({
    run: () => remove.mutateAsync(props.id),
    done: `${name} deleted.`,
    failed: `Couldn't delete ${name}.`,
    undo: () => restoreRole.mutateAsync(props.id),
    undone: `${name} restored.`,
    undoFailed: `Couldn't restore ${name}. It stays deleted.`,
  })
  if (ok) await router.push('/roles')
}
```

Replace `askRemove` with `removePerson` (rename its callers), and replace `addPerson`:

```ts
function removePerson(a: AccountListItem) {
  const before = a.roles.map((r) => r.id)
  const name = role.value?.name
  return runAction({
    run: () => assign.mutateAsync({ id: a.id, roleIds: before.filter((id) => id !== props.id) }),
    done: `${a.name} removed from ${name}.`,
    failed: `Couldn't remove ${a.name} from ${name}.`,
    undo: () => assign.mutateAsync({ id: a.id, roleIds: before }),
    undone: `${a.name} has ${name} again.`,
    undoFailed: `Couldn't give ${a.name} ${name} again.`,
  })
}
async function addPerson() {
  const a = candidates.value.find((x) => x.id === addId.value)
  if (!a) return
  const before = a.roles.map((r) => r.id)
  const name = role.value?.name
  const ok = await runAction({
    run: () => assign.mutateAsync({ id: a.id, roleIds: [...before, props.id] }),
    done: `${a.name} now has ${name}.`,
    failed: `Couldn't give ${a.name} ${name}.`,
    undo: () => assign.mutateAsync({ id: a.id, roleIds: before }),
    undone: `${a.name} removed from ${name} again.`,
    undoFailed: `Couldn't remove ${a.name} from ${name} again.`,
  })
  if (ok) {
    adding.value = false
    addId.value = ''
  }
}
```

Keep the Delete role button red (`danger`): deleting is a delete.

- [ ] **Step 3: Role dialogs → FormDialog**

Apply the recipe:
- **Edit role** (`editing`):
  - `size="m"`, `icon="shield"`, title `Edit role`, action `Save role`;
  - `busy` = `updateRole.isPending.value`;
  - dirty over name and description, with `form.reset()` at the end of `openEdit`.
- **Add people** (`adding`):
  - `size="s"`, `icon="user-plus"`, title `` `Add people to ${role.name}` ``, action `Add`;
  - `:disabled="!addId"`, `busy` = `assign.isPending.value`;
  - no `error` (runAction reports it);
  - dirty: `addId !== ''`.
- **RolesPage.vue** New role (`creating`):
  - `size="m"`, `icon="shield"`, title `New role`, action `Create role`;
  - dirty over its fields, with `form.reset()` where the dialog opens.

- [ ] **Step 4: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/roles
git commit -m "feat(frontend): role delete, permissions and membership offer Undo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Export profiles and the export dialogs

**Files:**
- Modify:
  - `frontend/src/features/assets/export/api.ts`
  - `frontend/src/features/export-profiles/pages/ExportProfilesPage.vue`
  - `frontend/src/features/assets/export/components/ReportDialog.vue` (check for user edits first, per Global Constraints)
  - `frontend/src/features/assets/export/components/DataExportDialog.vue`

**Interfaces:**
- Consumes: `runAction`, `announce` (Task 4); `FormDialog` (Task 5); `POST /export-profiles/{profileID}/restore` (Plan A).
- Produces: `useRestoreExportProfile()` with variables `id: string`.

- [ ] **Step 1: API**

In `features/assets/export/api.ts`:
- change `useDeleteExportProfile` to pass `false` as the second argument of `useProfileMutation`;
- add:

```ts
// Undo của xoá profile (xoá mềm)
export function useRestoreExportProfile() {
  return useProfileMutation(
    (id: string) => unwrap(inventoryApi.POST('/export-profiles/{profileID}/restore', { params: { path: { profileID: id } } })),
    false,
  )
}
```

- [ ] **Step 2: ExportProfilesPage**

Remove `useConfirm` and `const confirm`. Import `runAction` and `useRestoreExportProfile`. Change `const update = useUpdateExportProfile()` to `useUpdateExportProfile(false)`, and add `const restore = useRestoreExportProfile()`.

Replace `saveRename`, `toggleShare` and `askDelete`. Rename `askDelete(` callers to `deleteProfile(`.

```ts
async function saveRename(p: ExportProfile) {
  const name = newName.value.trim()
  if (!name || name === p.name) {
    cancelRename()
    return
  }
  const ok = await runAction({
    run: () => update.mutateAsync({ id: p.id, version: p.version, name }),
    done: `${p.name} renamed to ${name}.`,
    failed: `Couldn't rename ${p.name}.`,
    undo: (next) => update.mutateAsync({ id: p.id, version: next.version, name: p.name }),
    undone: `Name put back to ${p.name}.`,
    undoFailed: `Couldn't put the name back. It stays ${name}.`,
  })
  // lỗi (trùng tên, đã bị sửa): giữ ô nhập để sửa tiếp
  if (ok) renaming.value = null
}

function toggleShare(p: ExportProfile) {
  const sharing = !p.shared
  return runAction({
    run: () => update.mutateAsync({ id: p.id, version: p.version, shared: sharing }),
    done: sharing ? `${p.name} is shared with everyone who can export.` : `${p.name} is private again.`,
    failed: `Couldn't change who sees ${p.name}.`,
    undo: (next) => update.mutateAsync({ id: p.id, version: next.version, shared: p.shared }),
    undone: sharing ? `${p.name} is private again.` : `${p.name} is shared again.`,
    undoFailed: `Couldn't change who sees ${p.name} back.`,
  })
}

function deleteProfile(p: ExportProfile) {
  return runAction({
    run: () => remove.mutateAsync(p.id),
    done: `${p.name} deleted.`,
    failed: `Couldn't delete ${p.name}.`,
    undo: () => restore.mutateAsync(p.id),
    undone: `${p.name} restored.`,
    undoFailed: `Couldn't restore ${p.name}. It stays deleted.`,
  })
}
```

- [ ] **Step 3: Report: saving a profile offers Undo**

In `ReportDialog.vue`, import `announce`. In `saveHere()`:
- capture `const before = { name: p.name, shared: p.shared, layout: saved.value }` before the mutation;
- replace the `notify.success(...)` line with:

```ts
    announce(next, {
      done: ch.name ? `Saved ${next.name} (renamed).` : `Saved ${next.name}.`,
      undo: async (n) => {
        const back = await update.mutateAsync({ id: p.id, version: n.version, name: before.name, shared: before.shared, layout: before.layout })
        // hộp thoại còn mở trên profile này: lấy bản vừa đặt lại
        if (profileId.value === p.id) {
          saved.value = normalizeLayout(back.layout)
          loadedVersion.value = back.version
        }
      },
      undone: `${before.name} put back as it was.`,
      undoFailed: `Couldn't put ${before.name} back. The saved version stays.`,
    })
```

- [ ] **Step 4: Report and Data export → FormDialog**

- **ReportDialog.vue:**
  - Replace `<Dialog … class="report-dialog" …>` with:

    ```vue
    <FormDialog
      v-model:visible="visible"
      icon="file"
      title="Export report"
      width="max(80rem, 88vw)"
      flush
      :dirty="dirty"
      class="report-dialog"
    >
    ```

  - Move the contents of the old `#header` slot, except the `<span class="p-dialog-title">Export report</span>`, into `<template #header-extra>`. That covers the profile Select, the lock/shared icons, the Unsaved Tag and the Save… button.
  - Keep the old `#footer` content as `<template #footer>…</template>`. FormDialog's `footer` slot replaces its buttons.
  - Remove the `:pt` footer style, the `:content-style` and the `:style` width; FormDialog supplies them.
  - Width ruling: the report keeps its current width. The spec says L, but the report's three panes need the width the user tuned.
- **DataExportDialog.vue:**
  - `<FormDialog v-model:visible="visible" size="l" icon="file" title="What’s in the data export" flush>`;
  - keep the `#footer` slot (file name and Done).
- **Report panes:** in `ReportDialog.vue`'s styles, add `scrollbar-gutter: stable;` to each pane rule that has `overflow: auto` or `overflow-y: auto` (the configuration side and the preview).
- **Save… popover:** add the lead line as the first child of `<form class="save-as">`, with the same style as the filter popover's `.pop-lead`. Its buttons already go Cancel, then the actions.

  ```vue
  <p class="pop-lead">Keep this layout as a profile to reuse it on any asset list.</p>
  ```

- [ ] **Step 5: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/features/assets/export frontend/src/features/export-profiles
git commit -m "feat(frontend): export profile delete, share, rename and layout save offer Undo

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Tabs — close with Undo, unsaved close confirmed

**Files:**
- Modify:
  - `frontend/src/app/tabs/tabList.ts`
  - `frontend/src/app/tabs/useTabs.ts`
  - `frontend/src/app/layouts/TabBar.vue` (check for user edits first, per Global Constraints)
- Test: `frontend/src/app/tabs/tabList.test.ts`

**Interfaces:**
- Consumes: `runAction` (Task 4); `confirmAction`, `FormDialog`, `useDirty` (Task 5).
- Produces:
  - `reopenTab(tabs: Tab[], tab: Tab, index: number): Tab[]`;
  - `useTabs().reopen(closed: Tab, index: number, activate: boolean): Promise<void>`.

- [ ] **Step 1: Write the failing test**

Append to `frontend/src/app/tabs/tabList.test.ts`, and add `reopenTab` to the import from `./tabList`:

```ts
describe('reopenTab', () => {
  const t = (id: string, pinned = false) => ({ id, path: `/${id}`, pinned })
  it('puts the tab back at its old index', () => {
    expect(reopenTab([t('a'), t('c')], t('b'), 1).map((x) => x.id)).toEqual(['a', 'b', 'c'])
  })
  it('clamps an index past the end', () => {
    expect(reopenTab([t('a')], t('b'), 5).map((x) => x.id)).toEqual(['a', 'b'])
  })
  it('never lands inside the pinned group', () => {
    expect(reopenTab([t('p', true), t('q', true), t('a')], t('b'), 0).map((x) => x.id)).toEqual(['p', 'q', 'b', 'a'])
  })
})
```

- [ ] **Step 2: Run it to see it fail**

Run: `npx vitest run src/app/tabs/tabList.test.ts`
Expected: FAIL, because `reopenTab` isn't exported.

- [ ] **Step 3: Implement**

Append to `frontend/src/app/tabs/tabList.ts`:

```ts
// reopenTab: Undo của đóng tab: đặt lại ở vị trí cũ, không chen vào nhóm ghim, không quá cuối
export function reopenTab(tabs: Tab[], tab: Tab, index: number): Tab[] {
  const at = Math.min(Math.max(index, pinnedCount(tabs)), tabs.length)
  return [...tabs.slice(0, at), tab, ...tabs.slice(at)]
}
```

In `useTabs.ts`:
- add `reopenTab` to the `./tabList` import;
- add after `close`:

```ts
  // reopen: Undo của close; tab mới (id mới) cùng đường dẫn và vị trí, mở lại nếu nó đang mở
  function reopen(closed: Tab, index: number, wasActive: boolean) {
    const tab: Tab = { ...closed, id: newId(), pinned: false }
    tabs.value = reopenTab(tabs.value, tab, index)
    if (!wasActive) return Promise.resolve()
    activeId.value = null
    return activate(tab.id)
  }
```

- add `reopen` to the returned object.

- [ ] **Step 4: Run it to see it pass**

Run: `npx vitest run src/app/tabs/tabList.test.ts`
Expected: PASS.

- [ ] **Step 5: TabBar**

Remove `useConfirm` and `const confirm`. Import `runAction` (`@/lib/actions`), `confirmAction` (`@/lib/confirm`), `FormDialog` and `useDirty`.

Replace `close`:

```ts
// đóng tab: có Undo; tab còn thay đổi chưa lưu thì hỏi trước (bỏ thay đổi không hoàn tác được)
async function close(t: TabItem) {
  const name = label(t)
  if (tabs.dirty.has(t.id)) {
    const ok = await confirmAction({
      title: `Close ${name}?`,
      body: 'This tab has unsaved changes. Closing it discards them.',
      action: 'Discard and close',
      danger: true,
      icon: 'alert',
    })
    if (ok) await tabs.close(t.id)
    return
  }
  const index = tabs.tabs.findIndex((x) => x.id === t.id)
  const wasActive = tabs.activeId === t.id
  const snapshot = { ...t }
  await runAction({
    run: () => tabs.close(t.id),
    done: `${name} closed.`,
    undo: () => tabs.reopen(snapshot, index, wasActive),
    undone: `${name} reopened.`,
  })
}
```

- **Rename tab dialog:** apply the recipe.
  - `size="s"`, `icon="file"`, title `Rename tab`, action `Rename`;
  - no `busy`/`error`;
  - `form = useDirty(() => renameText.value)`, with `form.reset()` at the end of `startRename`;
  - submit calls the existing save-rename function.
- **Style:** in `.app-tab.p-tab-active`, delete the line `box-shadow: inset 0 2px 0 var(--app-accent);`. That one-sided stripe goes; the active tab keeps its background and bold label.

- [ ] **Step 6: Check and commit**

Run: `npm run check`
Expected: pass.

```bash
git add frontend/src/app/tabs/tabList.ts frontend/src/app/tabs/tabList.test.ts frontend/src/app/tabs/useTabs.ts frontend/src/app/layouts/TabBar.vue
git commit -m "feat(frontend): closing a tab offers Undo; unsaved tabs confirm first

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: Docs, sweep, browser walkthrough

**Files:**
- Modify: `frontend/README.md`

- [ ] **Step 1: Sweep for leftovers**

Run each of these from `frontend/`:
- `grep -rn "useConfirm\|confirm.require" src --include=*.vue --include=*.ts`
  Expected: only `src/App.vue` (`bindConfirm(useConfirm().require)`).
- `grep -rln "from 'primevue/dialog'" src`
  Expected: only `src/components/FormDialog.vue`, `src/app/layouts/AppLayout.vue` (keyboard help) and `src/app/layouts/TypeSwitcher.vue`.
- `grep -rn "app-bar-left\|inset 2px 0 0\|inset 0 2px 0" src`
  Expected: no output.

Fix anything else these find before going on.

- [ ] **Step 2: README**

Add a section `## Interaction patterns` to `frontend/README.md`, after `## Rules worth knowing`:

```markdown
## Interaction patterns

Spec: `docs/superpowers/specs/2026-10-07-ui-patterns-design.md`.

- **Act now, offer Undo.**
  - Use `runAction({ run, done, undo?, undone?, undoFailed?, failed? })` from `lib/actions.ts` for anything the server can reverse.
  - `undo` receives `run`'s result, for example the new `version`.
  - Use `announce(result, …)` when a form already ran the mutation and shows its own errors.
  - Mutations used this way are created with `toast: false`.
- **Ask only before the irreversible:**
  - `confirmAction({ title, body, impact?, action, danger, icon })` from `lib/confirm.ts`.
  - Red (`danger`) only for discarding; blue for sends and sign-outs.
- **Toasts:**
  - `notify.success(msg, { undo?, action? })` and `notify.error(summary, { detail?, retry? })`;
  - success shows 4 s, or 8 s with Undo or an action; errors stay until closed;
  - at most three; Ctrl/⌘ Z runs the newest Undo.
- **Dialogs:**
  - `components/FormDialog.vue`, sizes `s`/`m`/`l`, with a filled `icon` from `components/icons.ts`;
  - pass `dirty` (from `useDirty` in `lib/forms.ts`) so Esc and ✕ ask before discarding.
- **Lists:** `EmptyState` and `TableSkeleton` in every DataTable `#empty` slot.
- **Colour:**
  - `--app-{brand,info,warn,danger,neutral}[-strong|-ink|-soft]` in `app/base.css`;
  - tags are solid via `lib/tones.ts`;
  - no coloured stripe on one side of anything.
```

- [ ] **Step 3: Full check**

Run: `npm run check`
Expected: pass.

Run: `cd ../backend && make check`
Expected: pass. Nothing in the backend changed; this confirms the generated types still match.

- [ ] **Step 4: Browser walkthrough**

This needs the dev app running on a database with the soft-delete columns (migrations 00002 and 00005 now create them; the dev database was reset and re-seeded on 2026-10-07). With the seeded data, check:
- **Assets:**
  - Retire an asset, then Undo.
  - Restore an asset, then Undo; it is retired again with its old reason.
  - Bulk retire 3 assets, then Undo once.
  - Bulk-change status with one asset already at the target, then Undo.
  - Edit and save an asset, then Undo.
- **Types:**
  - Archive a type, then Undo.
  - Remove an attribute, then Undo; reorder attributes, then Undo.
  - Remove an option, then Undo.
- **Statuses:** archive, restore, make default and reorder, each followed by Undo.
- **Accounts:**
  - Disable an account, then Undo; change roles, then Undo.
  - Resend invitation and Sign out everywhere each show the blue confirmation.
- **Roles:**
  - Remove a person from a role, then Undo; save permissions, then Undo.
  - Delete an empty custom role, then Undo; the role is back.
- **Export profiles:**
  - Delete, then Undo. Share, then Undo. Rename, then Undo.
  - Save a layout in the report dialog, then Undo.
- **Tabs:**
  - Close a tab, then Undo; it reopens in place.
  - Close a tab with unsaved changes; the red confirmation appears.
- **Shortcut:** Ctrl+Z in a text field undoes text only. Ctrl+Z elsewhere undoes the newest toast.
- **Errors:** stop the backend, then archive something. The error toast stays, shows its detail on hover, and Retry works once the backend is back.
- **Dialogs:** type in a dialog and press Esc; "Discard changes?" appears. Esc on an untouched dialog just closes it.
- **Dark theme:** tags, toasts, confirmation bubble and scrollbars look right.

- [ ] **Step 5: Commit**

```bash
git add frontend/README.md
git commit -m "docs(frontend): interaction patterns reference

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
