<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Chip from 'primevue/chip'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Panel from 'primevue/panel'
import Tab from 'primevue/tab'
import TabList from 'primevue/tablist'
import Tabs from 'primevue/tabs'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import { useTabDirty, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import DetailHeader from '@/components/DetailHeader.vue'
import SaveBar from '@/components/SaveBar.vue'
import { useRoles } from '@/features/roles/api'
import { ADMINISTRATOR_ROLE_ID, effective } from '@/features/roles/catalog'
import type { Role } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDateTime } from '@/lib/dates'
import { isApiError } from '@/lib/errors'
import { useFormErrors } from '@/lib/forms'
import { runAction } from '@/lib/actions'
import { notify } from '@/lib/notify'
import { initials, inviteNote, relativeTime } from '@/lib/people'
import { useUrlState } from '@/lib/urlState'
import { useAccount, useAssignRoles, useUpdateAccount } from '../api'
import { statusSeverity } from '../status'
import { useAccountActions } from '../useAccountActions'

// Trang tài khoản: thẻ đầu trang (ai, trạng thái, hành động) rồi Overview | Roles | Activity
const props = defineProps<{ id: string }>()

const session = useSession()
const canManage = computed(() => session.can(Perm.AccountManage))
const { data: account, refetch } = useAccount(() => props.id)
useTabTitle(() => account.value?.name)
const roles = useRoles()
const actions = useAccountActions()
const self = computed(() => !!account.value && actions.isSelf(account.value))

type Section = 'overview' | 'roles'
const { state, update } = useUrlState(
  (q) => ({ tab: (q.tab === 'roles' ? 'roles' : 'overview') as Section }),
  (s) => ({ tab: s.tab === 'roles' ? s.tab : undefined }),
)
const label = (s: string) => s[0].toUpperCase() + s.slice(1)

// Sửa tên: gửi version đã đọc; 409 là người khác vừa sửa
const name = ref('')
const nameErrors = useFormErrors()
const update_ = useUpdateAccount()
watch(
  () => account.value?.name,
  (n) => (name.value = n ?? ''),
  { immediate: true },
)
async function saveName() {
  if (!account.value) return
  nameErrors.clear()
  try {
    await update_.mutateAsync({ id: props.id, name: name.value, version: account.value.version })
    notify.success('Name saved.')
  } catch (err) {
    if (isApiError(err) && err.status === 409) {
      notify.info('Someone else changed this account. Reloaded the latest version.')
      await refetch()
    } else {
      nameErrors.set(err)
    }
  }
}

// Role: chọn bằng thẻ; bản nháp so với bản đã lưu, có thanh lưu khi khác
const saved = computed(() => account.value?.roles.map((r) => r.id) ?? [])
const draft = ref<string[] | null>(null)
const current = computed(() => draft.value ?? saved.value)
const dirty = computed(
  () => !!draft.value && (draft.value.length !== saved.value.length || draft.value.some((id) => !saved.value.includes(id))),
)
useTabDirty(() => dirty.value)
watch(saved, () => (draft.value = null))
function pick(id: string, on: boolean) {
  const set = new Set(current.value)
  if (on) set.add(id)
  else set.delete(id)
  draft.value = (roles.data.value ?? []).map((r) => r.id).filter((x) => set.has(x))
}
const grantable = (r: Role) => r.permissions.every((p) => session.can(p))
// Không tự bỏ role Administrator của mình (API trả 409); báo trước khi lưu
const lockout = computed(() => self.value && saved.value.includes(ADMINISTRATOR_ROLE_ID) && !current.value.includes(ADMINISTRATOR_ROLE_ID))
const groups = computed(() => effective(roles.data.value ?? [], saved.value, current.value))

const assign = useAssignRoles()
async function saveRoles() {
  const before = [...saved.value]
  await runAction({
    run: () => assign.mutateAsync({ id: props.id, roleIds: current.value }),
    done: `Roles of ${account.value?.name ?? 'the account'} saved.`,
    failed: "Couldn't save the roles.",
    undo: () => assign.mutateAsync({ id: props.id, roleIds: before }),
    undone: 'Roles put back.',
    undoFailed: "Couldn't put the roles back. The new roles stay.",
    after: () => (draft.value = null),
  })
}

const crumbs = computed(() => [{ label: 'Accounts', to: '/accounts' }, { label: account.value?.name ?? '…' }])
</script>

<template>
  <section v-if="account">
    <AppBreadcrumb :items="crumbs" />

    <DetailHeader :title="account.name">
      <template #media>
        <Avatar :label="initials(account.name)" shape="circle" size="xlarge" :class="['avatar', { muted: account.status === 'disabled' }]" />
      </template>
      <template #tags>
        <Tag :value="label(account.status)" :severity="statusSeverity(account.status)" />
        <Tag v-if="self" value="You" severity="secondary" />
      </template>
      <div>{{ account.email }}</div>
      <div class="chips">
        <Chip v-for="r in account.roles" :key="r.id" :label="r.name" icon="pi pi-shield" />
        <span v-if="!account.roles.length">No roles</span>
      </div>
      <template v-if="canManage" #actions>
        <Button v-if="account.status === 'invited'" label="Resend invitation" icon="pi pi-envelope" severity="secondary" outlined @click="actions.resendInvitation(account)" />
        <Button v-if="account.status === 'active'" label="Send reset link" icon="pi pi-key" severity="secondary" outlined @click="actions.sendReset(account)" />
        <Button v-if="account.status === 'disabled'" label="Enable" icon="pi pi-check-circle" @click="actions.enable(account)" />
        <span v-else v-tooltip.top="self ? 'You can’t disable yourself' : undefined">
          <Button label="Disable" icon="pi pi-ban" severity="secondary" outlined :disabled="self" @click="actions.disable(account)" />
        </span>
      </template>
    </DetailHeader>

    <Tabs :value="state.tab" class="section-tabs" @update:value="(v) => update({ tab: v as Section })">
      <TabList>
        <Tab value="overview">Overview</Tab>
        <Tab value="roles">Roles <span class="tab-count">{{ account.roles.length }}</span></Tab>
        <Tab value="activity" disabled>Activity · later</Tab>
      </TabList>
    </Tabs>

    <div v-if="state.tab === 'overview'" class="grid">
      <Panel header="Profile">
        <form class="form" @submit.prevent="saveName">
          <Message v-if="nameErrors.general.value" severity="error">{{ nameErrors.general.value }}</Message>
          <div class="field">
            <label for="acct-name">Display name</label>
            <div class="inline">
              <InputText id="acct-name" v-model="name" :disabled="!canManage" maxlength="200" />
              <Button v-if="canManage" type="submit" label="Save" severity="secondary" :loading="update_.isPending.value" :disabled="!name.trim() || name.trim() === account.name" />
            </div>
            <small v-if="nameErrors.fields.value.name" class="field-error">{{ nameErrors.fields.value.name }}</small>
          </div>
          <div class="field">
            <span>Email</span>
            <div>{{ account.email }}</div>
            <small>Used to sign in. It can't be changed here.</small>
          </div>
          <div class="field">
            <span>Linked member <Tag value="later" severity="secondary" class="later" /></span>
            <div class="muted">Not linked. Linking to a person in the directory comes with the directory module.</div>
          </div>
        </form>
      </Panel>
      <Panel header="Sign-in">
        <dl class="props">
          <dt>Created</dt>
          <dd>{{ formatDateTime(account.created_at) }}</dd>
          <dt>Last sign-in</dt>
          <dd>
            <template v-if="account.last_sign_in_at">
              {{ formatDateTime(account.last_sign_in_at) }} <span class="muted">({{ relativeTime(account.last_sign_in_at) }})</span>
            </template>
            <span v-else class="muted">Never</span>
          </dd>
          <dt>Active sessions</dt>
          <dd>{{ account.active_sessions ?? 0 }}</dd>
        </dl>
        <p v-if="account.status === 'invited'" class="note">
          {{ inviteNote(account.invite_expires_at) ?? 'No invitation link is pending.' }} Resending makes a new link and the
          old one stops working.
        </p>
        <Button
          v-if="canManage && account.status === 'active'"
          label="Sign out everywhere"
          icon="pi pi-sign-out"
          severity="secondary"
          outlined
          size="small"
          class="signout"
          :disabled="!account.active_sessions"
          @click="actions.signOutEverywhere(account)"
        />
      </Panel>
    </div>

    <div v-else class="grid">
      <Panel header="Roles">
        <Message v-if="lockout" severity="warn" :closable="false" class="block-msg">
          You can't remove your own Administrator role. Ask another administrator to do it.
        </Message>
        <div class="role-pick">
          <label
            v-for="r in roles.data.value ?? []"
            :key="r.id"
            class="role-option"
            :class="{ on: current.includes(r.id), off: !canManage || !grantable(r) }"
          >
            <Checkbox
              :model-value="current.includes(r.id)"
              binary
              :input-id="`pick-${r.id}`"
              :disabled="!canManage || !grantable(r)"
              @update:model-value="(v: boolean) => pick(r.id, v)"
            />
            <span>
              <strong>{{ r.name }}</strong>
              <small>{{ grantable(r) ? `${r.permissions.length} permissions` : 'Has permissions you don’t hold' }}</small>
            </span>
          </label>
        </div>
        <p class="note">You can only hand out permissions you hold yourself.</p>
      </Panel>
      <Panel header="What they can do">
        <div v-for="g in groups" :key="g.module" class="perm-group">
          <div class="eyebrow">{{ g.module }}</div>
          <ul class="perm-list">
            <li v-for="p in g.items" :key="p.code" :class="p.state">
              <i :class="p.state === 'removed' ? 'pi pi-minus' : p.state === 'added' ? 'pi pi-plus' : 'pi pi-check'" />
              <span>{{ p.label }}</span>
            </li>
          </ul>
        </div>
        <p v-if="!groups.length" class="muted">No permissions. They can sign in but see nothing.</p>
      </Panel>
      <SaveBar
        v-if="dirty"
        :message="`Roles changed for ${account.name}`"
        save-label="Save roles"
        :saving="assign.isPending.value"
        :blocked="lockout"
        @save="saveRoles"
        @discard="draft = null"
      />
    </div>
  </section>
</template>

<style scoped>
.avatar {
  flex: none;
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
  font-weight: 700;
}
.avatar.muted {
  background: var(--app-soft);
  color: var(--p-text-muted-color);
}
.muted {
  color: var(--p-text-muted-color);
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}
.chips :deep(.p-chip) {
  padding: 0.1rem 0.6rem;
  font-size: 0.8rem;
}
.section-tabs {
  margin-bottom: 1rem;
}
.tab-count {
  font: 0.75rem var(--app-mono);
  color: var(--p-text-muted-color);
}
.grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: 1rem;
  align-items: start;
}
@media (max-width: 900px) {
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
.inline {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}
.inline .p-inputtext {
  flex: 1;
  min-width: 10rem;
}
.later {
  /* nhãn nhỏ cạnh chữ: không theo bề rộng chung của tag */
  min-width: 0;
  font-size: 0.65rem;
  margin-left: 0.3rem;
}
.note {
  margin: 0.75rem 0 0;
  font-size: 0.85rem;
  color: var(--p-text-muted-color);
}
.signout {
  margin-top: 0.9rem;
}
.block-msg {
  margin-bottom: 0.75rem;
}
.role-pick {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr));
  gap: 0.6rem;
}
.role-option {
  display: flex;
  align-items: flex-start;
  gap: 0.6rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid var(--app-line);
  border-radius: 9px;
  cursor: pointer;
}
.role-option:hover {
  background: var(--p-list-option-focus-background);
}
/* role đang chọn: nền xám như mục đang chọn ở nơi khác; checkbox đã tick nói phần còn lại */
.role-option.on {
  background: var(--app-selected);
  border-color: transparent;
}
.role-option.off {
  cursor: default;
}
.role-option span {
  display: flex;
  flex-direction: column;
}
.role-option small {
  color: var(--p-text-muted-color);
}
.perm-group + .perm-group {
  margin-top: 0.9rem;
}
.perm-list {
  list-style: none;
  margin: 0.35rem 0 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.perm-list li {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
}
.perm-list i {
  font-size: 0.75rem;
  color: var(--app-accent);
}
.perm-list li.added {
  color: var(--app-accent);
  font-weight: 600;
}
.perm-list li.removed {
  color: var(--p-red-500);
  text-decoration: line-through;
}
.perm-list li.removed i {
  color: var(--p-red-500);
}
</style>
