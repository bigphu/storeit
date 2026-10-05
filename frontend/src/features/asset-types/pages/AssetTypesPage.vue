<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
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
import { useAssetTypes, useCreateAssetType } from '../api'

const session = useSession()
const router = useRouter()
const showArchived = ref(false)
const { data: types, isFetching } = useAssetTypes(showArchived)

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
    await router.push(`/asset-types/${t.id}`)
  } catch (err) {
    errors.set(err)
  }
}
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
    <DataTable :value="types ?? []" :loading="isFetching" data-key="id">
      <Column header="Name">
        <template #body="{ data: t }: { data: AssetType }">
          <RouterLink :to="`/asset-types/${t.id}`">{{ t.name }}</RouterLink>
          <Tag v-if="t.archived_at" value="archived" severity="secondary" class="ml" />
        </template>
      </Column>
      <Column field="code" header="Code" />
      <Column field="description" header="Description" />
      <Column header="Assets">
        <template #body="{ data: t }: { data: AssetType }">
          <RouterLink :to="{ path: '/assets', query: { type_id: t.id } }">View assets</RouterLink>
        </template>
      </Column>
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
</style>
