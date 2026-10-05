<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref } from 'vue'
import type { Status, StatusKind } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { kindSeverity, statusKinds, useArchiveStatus, useCreateStatus, useStatuses, useUpdateStatus } from '../api'

const session = useSession()
const confirm = useConfirm()
const canManage = computed(() => session.can(Perm.StatusManage))

const showArchived = ref(false)
const { data: statuses, isFetching } = useStatuses(showArchived)

// Một dialog cho cả tạo và sửa; editing null là tạo mới
const open = ref(false)
const editing = ref<Status | null>(null)
const name = ref('')
const kind = ref<StatusKind>('available')
const position = ref(0)
const makeDefault = ref(false)
const errors = useFormErrors()
const create = useCreateStatus()
const update = useUpdateStatus()

function openCreate() {
  editing.value = null
  name.value = ''
  kind.value = 'available'
  position.value = 0
  makeDefault.value = false
  errors.clear()
  open.value = true
}

function openEdit(s: Status) {
  editing.value = s
  name.value = s.name
  kind.value = s.kind
  position.value = s.position
  makeDefault.value = false
  errors.clear()
  open.value = true
}

async function submit() {
  errors.clear()
  try {
    if (editing.value) {
      await update.mutateAsync({
        id: editing.value.id,
        name: name.value,
        position: position.value,
        make_default: makeDefault.value || undefined,
      })
    } else {
      await create.mutateAsync({ name: name.value, kind: kind.value, position: position.value })
    }
    open.value = false
    notify.success('Status saved.')
  } catch (err) {
    errors.set(err)
  }
}

const archive = useArchiveStatus()
function askArchive(s: Status) {
  confirm.require({
    message: `Archive the status ${s.name}? Assets that use it keep it.`,
    header: 'Confirm',
    acceptLabel: 'Archive',
    rejectLabel: 'Cancel',
    accept: () =>
      archive
        .mutateAsync(s.id)
        .then(() => notify.success('Status archived.'))
        .catch(() => {}),
  })
}
</script>

<template>
  <section>
    <div class="page-header">
      <h1>Statuses</h1>
      <Button v-if="canManage" label="New status" icon="pi pi-plus" @click="openCreate" />
    </div>
    <div class="toolbar">
      <Checkbox v-model="showArchived" input-id="show-archived" binary />
      <label for="show-archived">Show archived</label>
    </div>
    <DataTable :value="statuses ?? []" :loading="isFetching" data-key="id">
      <Column field="name" header="Name" />
      <Column header="Kind">
        <template #body="{ data: s }: { data: Status }">
          <Tag :value="s.kind" :severity="kindSeverity(s.kind)" />
        </template>
      </Column>
      <Column header="Default">
        <template #body="{ data: s }: { data: Status }">{{ s.is_default ? 'Yes' : '' }}</template>
      </Column>
      <Column header="Built-in">
        <template #body="{ data: s }: { data: Status }">{{ s.is_system ? 'Yes' : '' }}</template>
      </Column>
      <Column field="position" header="Position" />
      <Column header="Archived">
        <template #body="{ data: s }: { data: Status }">{{ s.archived_at ? 'Yes' : '' }}</template>
      </Column>
      <Column v-if="canManage" header="">
        <template #body="{ data: s }: { data: Status }">
          <div class="actions">
            <Button label="Edit" size="small" text @click="openEdit(s)" />
            <Button
              v-if="!s.archived_at && !s.is_system"
              label="Archive"
              size="small"
              text
              severity="danger"
              @click="askArchive(s)"
            />
          </div>
        </template>
      </Column>
    </DataTable>

    <Dialog v-model:visible="open" modal :header="editing ? 'Edit status' : 'New status'" :style="{ width: '30rem' }">
      <form class="form" @submit.prevent="submit">
        <Message v-if="errors.general.value" severity="error">{{ errors.general.value }}</Message>
        <div class="field">
          <label for="status-name">Name</label>
          <InputText id="status-name" v-model="name" required />
          <small v-if="errors.fields.value.name" class="field-error">{{ errors.fields.value.name }}</small>
        </div>
        <div class="field">
          <label for="status-kind">Kind</label>
          <!-- kind không đổi được sau khi tạo -->
          <Select
            v-model="kind"
            input-id="status-kind"
            :options="statusKinds"
            :disabled="!!editing"
          />
          <small v-if="errors.fields.value.kind" class="field-error">{{ errors.fields.value.kind }}</small>
        </div>
        <div class="field">
          <label for="status-position">Position</label>
          <InputNumber v-model="position" input-id="status-position" :use-grouping="false" />
        </div>
        <div v-if="editing && !editing.is_default" class="actions">
          <Checkbox v-model="makeDefault" input-id="status-default" binary />
          <label for="status-default">Make this the default for “{{ editing.kind }}”</label>
        </div>
        <div class="actions">
          <Button type="submit" label="Save" :loading="create.isPending.value || update.isPending.value" />
          <Button label="Cancel" severity="secondary" text @click="open = false" />
        </div>
      </form>
    </Dialog>
  </section>
</template>
