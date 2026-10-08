<script setup lang="ts">
import Button from 'primevue/button'
import { onMounted, onUnmounted, ref } from 'vue'
import { countdown, holds } from '@/lib/countdown'
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
// chuột và focus là hai lý do dừng riêng: chỉ chạy lại khi không còn cái nào
const hold = holds(
  () => timer?.pause(performance.now()),
  () => timer?.resume(performance.now()),
)

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
    @mouseenter="hold.hold('hover')"
    @mouseleave="hold.release('hover')"
    @focusin="hold.hold('focus')"
    @focusout="(e: FocusEvent) => !(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node) && hold.release('focus')"
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
