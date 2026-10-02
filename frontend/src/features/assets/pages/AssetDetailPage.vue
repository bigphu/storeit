<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref } from 'vue'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDate, formatDateTime } from '@/lib/dates'
import { notify } from '@/lib/notify'
import { kindSeverity } from '@/features/statuses/api'
import { useAsset, useRestoreAsset, useRetireAsset } from '../api'
import { formatValue } from '../values'

const props = defineProps<{ id: string }>()

const session = useSession()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.AssetManage))

const { data: asset } = useAsset(() => props.id)
const retired = computed(() => !!asset.value?.retired_at)

const retireOpen = ref(false)
const reason = ref('')
const retire = useRetireAsset()
function openRetire() {
  reason.value = ''
  retireOpen.value = true
}
async function submitRetire() {
  if (!asset.value) return
  await retire
    .mutateAsync({ id: props.id, reason: reason.value, version: asset.value.version })
    .then(() => {
      retireOpen.value = false
      notify.success('Asset retired.')
    })
    .catch(() => {})
}

const restore = useRestoreAsset()
function askRestore() {
  const a = asset.value
  if (!a) return
  confirm.require({
    message: `Restore ${a.tag}? It goes back to the default available status.`,
    header: 'Confirm',
    acceptLabel: 'Restore',
    rejectLabel: 'Cancel',
    accept: () =>
      restore
        .mutateAsync({ id: a.id, version: a.version })
        .then(() => notify.success('Asset restored.'))
        .catch(() => {}),
  })
}
</script>

<template>
  <section v-if="asset">
    <div class="page-header">
      <h1>{{ asset.tag }} — {{ asset.name }}</h1>
      <div v-if="canManage" class="actions">
        <Button v-if="!retired" as="router-link" :to="`/assets/${asset.id}/edit`" label="Edit" icon="pi pi-pencil" />
        <Button
          v-if="!retired"
          label="Retire"
          severity="danger"
          text
          @click="openRetire"
        />
        <Button v-else label="Restore" @click="askRestore" />
      </div>
    </div>

    <dl class="props">
      <dt>Type</dt>
      <dd>
        <RouterLink :to="`/asset-types/${asset.asset_type.id}`">{{ asset.asset_type.name }}</RouterLink>
      </dd>
      <dt>Status</dt>
      <dd><Tag :value="asset.status.name" :severity="kindSeverity(asset.status.kind)" /></dd>
      <dt>Description</dt>
      <dd class="pre">{{ asset.description || '—' }}</dd>
      <dt>Purchase date</dt>
      <dd>{{ formatDate(asset.purchase_date) }}</dd>
      <template v-if="asset.retired_at">
        <dt>Retired</dt>
        <dd>{{ formatDateTime(asset.retired_at) }}<template v-if="asset.retired_reason"> — {{ asset.retired_reason }}</template></dd>
      </template>
      <dt>Created</dt>
      <dd>{{ formatDateTime(asset.created_at) }}</dd>
      <dt>Updated</dt>
      <dd>{{ formatDateTime(asset.updated_at) }}</dd>
    </dl>

    <section>
      <h2>Attributes</h2>
      <dl v-if="asset.attributes.length" class="props">
        <template v-for="a in asset.attributes" :key="a.key">
          <dt>{{ a.label }}</dt>
          <dd class="pre">{{ formatValue(a) }}</dd>
        </template>
      </dl>
      <p v-else>This asset type has no attributes.</p>
    </section>

    <Dialog v-model:visible="retireOpen" modal header="Retire asset" :style="{ width: '30rem' }">
      <form class="form" @submit.prevent="submitRetire">
        <p>Retired assets are hidden from the list and can't be edited until restored.</p>
        <div class="field">
          <label for="retire-reason">Reason (optional)</label>
          <Textarea id="retire-reason" v-model="reason" rows="3" />
        </div>
        <div class="actions">
          <Button type="submit" label="Retire" severity="danger" :loading="retire.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="retireOpen = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>

<style scoped>
.pre {
  white-space: pre-wrap;
}
</style>
