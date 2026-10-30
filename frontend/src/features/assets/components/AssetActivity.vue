<script setup lang="ts">
import Button from 'primevue/button'
import Timeline from 'primevue/timeline'
import { computed, reactive } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import TableSkeleton from '@/components/TableSkeleton.vue'
import { formatDateTime } from '@/lib/dates'
import { activitySummary, type ActivityEntry, isLong, relativeTime } from '../activity'
import { useAssetActivity } from '../api'

// Tab Activity của trang tài sản: dòng thời gian mới nhất trước, mỗi lần sửa kèm từng trường
// đổi (trước → sau). Tải khi tab mở lần đầu (cha chỉ dựng component lúc đó)
const props = defineProps<{ assetId: string }>()

const { data, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useAssetActivity(
  () => props.assetId,
  true,
)
const entries = computed<ActivityEntry[]>(() => data.value?.pages.flatMap((p) => p.items) ?? [])
const now = new Date()

const ICONS: Record<ActivityEntry['kind'], string> = {
  created: 'pi pi-plus-circle',
  updated: 'pi pi-pencil',
  retired: 'pi pi-ban',
  restored: 'pi pi-replay',
  replaced: 'pi pi-sync',
  replaces: 'pi pi-sync',
  replacement_undone: 'pi pi-undo',
  reserved: 'pi pi-bookmark',
  reservation_cancelled: 'pi pi-times-circle',
  lent: 'pi pi-arrow-right-arrow-left',
  other: 'pi pi-circle',
}

// câu tóm tắt tách quanh tag của tài sản liên quan (thay thế) để tag thành liên kết
function summaryParts(item: ActivityEntry): { text: string; link: boolean }[] {
  const text = activitySummary(item).text
  const tag = item.related?.tag
  const at = tag ? text.indexOf(tag) : -1
  if (!tag || at < 0) return [{ text, link: false }]
  return [
    { text: text.slice(0, at), link: false },
    { text: tag, link: true },
    { text: text.slice(at + tag.length), link: false },
  ]
}

// giá trị dài đang mở rộng, theo "<id entry>:<field>:<from|to>"
const expanded = reactive(new Set<string>())
function toggle(key: string) {
  if (expanded.has(key)) expanded.delete(key)
  else expanded.add(key)
}
</script>

<template>
  <TableSkeleton v-if="isLoading" :rows="4" />
  <EmptyState v-else-if="isError" icon="pi pi-exclamation-circle" text="Couldn't load the activity." action="Retry" @action="refetch()" />
  <EmptyState v-else-if="!entries.length" icon="pi pi-history" text="No activity recorded yet." />
  <div v-else class="activity">
    <Timeline :value="entries" class="timeline">
      <template #marker="{ item }: { item: ActivityEntry }">
        <span class="marker" :class="item.kind"><i :class="ICONS[item.kind]" aria-hidden="true" /></span>
      </template>
      <template #content="{ item }: { item: ActivityEntry }">
        <div class="entry">
          <p class="summary">
            <strong>{{ activitySummary(item).who }}</strong>
            <template v-for="(part, i) in summaryParts(item)" :key="i">
              <RouterLink v-if="part.link && item.related" :to="`/assets/${item.related.id}`" class="mono">{{ part.text }}</RouterLink>
              <template v-else>{{ ` ${part.text}`.replace(/^ (?=:)/, '') }}</template>
            </template>
            <time class="when" :datetime="item.occurred_at" :title="formatDateTime(item.occurred_at)">{{ relativeTime(item.occurred_at, now) }}</time>
          </p>
          <dl v-if="item.changes.length" class="changes">
            <template v-for="c in item.changes" :key="c.field">
              <dt>{{ c.label }}</dt>
              <dd>
                <template v-for="side in (['from', 'to'] as const)" :key="side">
                  <span v-if="side === 'to'" class="arrow" aria-label="changed to">→</span>
                  <span class="value" :class="{ empty: c[side] === null, clamp: isLong(c[side]) && !expanded.has(`${item.id}:${c.field}:${side}`) }">{{
                    c[side] ?? '—'
                  }}</span>
                  <Button
                    v-if="isLong(c[side])"
                    :label="expanded.has(`${item.id}:${c.field}:${side}`) ? 'Show less' : 'Show more'"
                    text
                    size="small"
                    class="more"
                    @click="toggle(`${item.id}:${c.field}:${side}`)"
                  />
                </template>
              </dd>
            </template>
          </dl>
        </div>
      </template>
    </Timeline>
    <Button v-if="hasNextPage" label="Load more" icon="pi pi-angle-down" text :loading="isFetchingNextPage" class="load-more" @click="fetchNextPage()" />
  </div>
</template>

<style scoped>
.activity {
  display: grid;
  gap: 0.5rem;
  max-width: 56rem;
}
/* chỉ một cột nội dung bên phải dấu mốc */
.timeline :deep(.p-timeline-event-opposite) {
  display: none;
}
.marker {
  display: inline-grid;
  place-items: center;
  width: 2rem;
  height: 2rem;
  border-radius: 50%;
  background: var(--app-soft);
  color: var(--p-text-muted-color);
}
.marker.retired {
  color: var(--p-text-color);
}
.entry {
  padding-bottom: 1rem;
}
.summary {
  margin: 0.25rem 0 0;
}
.when {
  margin-left: 0.5rem;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.changes {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: 0.25rem 1rem;
  margin: 0.5rem 0 0;
  font-size: 0.9rem;
}
.changes dt {
  color: var(--p-text-muted-color);
}
.changes dd {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.35rem;
  min-width: 0;
}
.value {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.value.empty {
  color: var(--p-text-muted-color);
}
.value.clamp {
  display: -webkit-box;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.arrow {
  color: var(--p-text-muted-color);
}
.more {
  padding: 0 0.25rem;
}
.load-more {
  justify-self: start;
}
</style>
