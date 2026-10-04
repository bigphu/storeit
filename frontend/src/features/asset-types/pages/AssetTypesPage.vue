<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import ContextMenu from 'primevue/contextmenu'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tag from 'primevue/tag'
import Textarea from 'primevue/textarea'
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import type { AssetType } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { openLocation } from '@/lib/navigation'
import { onRowClick, useRowMenu } from '@/lib/tableRows'
import { useAssetTypes, useCreateAssetType } from '../api'

const session = useSession()
const router = useRouter()
const showArchived = ref(false)
const { data: types, isFetching } = useAssetTypes(showArchived, true)

const creating = ref(false)
const code = ref('')
const name = ref('')
const description = ref('')
const errors = useFormErrors()
const create = useCreateAssetType()

function openCreate() {
  code.value = name.value = description.value = ''
  errors.clear()
  creating.value = true
}

async function submit() {
  errors.clear()
  try {
    const t = await create.mutateAsync({ code: code.value, name: name.value, description: description.value })
    creating.value = false
    // thêm thuộc tính ở trang chi tiết
    await router.push(`/types/${t.id}/settings`)
  } catch (err) {
    errors.set(err)
  }
}

// Bấm dòng mở danh sách tài sản của loại (Ctrl/⌘ mở tab mới); chuột phải có thêm cài đặt
function openType(t: AssetType, e?: MouseEvent, newTab?: boolean) {
  openLocation(router, `/types/${t.id}/assets`, e, newTab)
}
const rowClick = onRowClick(openType)
const menu = ref<InstanceType<typeof ContextMenu>>()
const { items: menuItems, show: showMenu, clear: clearMenu } = useRowMenu<AssetType>(menu, (t) => [
  { label: 'Open assets', icon: 'pi pi-arrow-right', command: () => openType(t) },
  { label: 'Open in new tab', icon: 'pi pi-external-link', command: () => openType(t, undefined, true) },
  { separator: true },
  { label: 'Type settings', icon: 'pi pi-cog', command: () => openLocation(router, `/types/${t.id}/settings`) },
])
</script>

<template>
  <section>
    <div class="page-header">
      <h1>Asset types</h1>
      <Button v-if="session.can(Perm.TypeManage)" label="New type" icon="pi pi-plus" @click="openCreate" />
    </div>
    <div class="toolbar">
      <Checkbox v-model="showArchived" input-id="show-archived" binary />
      <label for="show-archived">Show archived</label>
    </div>
    <ContextMenu ref="menu" :model="menuItems" @hide="clearMenu" />
    <DataTable
      :value="types ?? []"
      :loading="isFetching"
      data-key="id"
      removable-sort
      row-hover
      :row-class="() => 'clickable-row'"
      @row-click="rowClick"
      @row-contextmenu="showMenu"
    >
      <Column header="Name" sort-field="name" sortable>
        <template #body="{ data: t }: { data: AssetType }">
          <RouterLink :to="`/types/${t.id}/assets`">{{ t.name }}</RouterLink>
          <Tag v-if="t.archived_at" value="archived" severity="secondary" class="ml" />
        </template>
      </Column>
      <Column field="code" header="Code" sortable />
      <Column field="description" header="Description" sortable />
      <Column header="Assets" sort-field="asset_count" sortable>
        <template #body="{ data: t }: { data: AssetType }">{{ t.asset_count ?? '' }}</template>
      </Column>
      <Column header="" header-style="width: 4rem" body-class="end-cell">
        <!-- liên kết thật (Ctrl/chuột giữa mở tab mới), hiển thị như nút biểu tượng -->
        <template #body="{ data: t }: { data: AssetType }">
          <div class="row-actions">
            <Button v-slot="slot" v-tooltip.top="'Type settings'" icon="pi pi-cog" text rounded size="small" as-child>
              <RouterLink :to="`/types/${t.id}/settings`" :class="slot.class" aria-label="Type settings">
                <i class="pi pi-cog" />
              </RouterLink>
            </Button>
          </div>
        </template>
      </Column>
      <template #empty>No asset types yet.</template>
    </DataTable>

    <Dialog v-model:visible="creating" modal header="New asset type" :style="{ width: '32rem' }">
      <form class="form" @submit.prevent="submit">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
        <div class="field">
          <label for="type-code">Code</label>
          <InputText id="type-code" v-model="code" required placeholder="LAPTOP" />
          <small>A-Z, 0-9, _ or -. Can't be changed later.</small>
          <small v-if="errors.fields.value.code" class="field-error">{{ errors.fields.value.code }}</small>
        </div>
        <div class="field">
          <label for="type-name">Name</label>
          <InputText id="type-name" v-model="name" required />
          <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
        </div>
        <div class="field">
          <label for="type-desc">Description</label>
          <Textarea id="type-desc" v-model="description" rows="3" />
        </div>
        <div class="actions">
          <Button type="submit" label="Create" :loading="create.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="creating = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>

<style scoped>
.ml {
  margin-left: 0.5rem;
}
:deep(.end-cell) {
  text-align: right;
}
</style>
