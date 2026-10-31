<script setup lang="ts">
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import Message from 'primevue/message'
import Panel from 'primevue/panel'
import Tag from 'primevue/tag'
import { computed, reactive, ref, watch } from 'vue'
import { useLeaveGuard, useTabDirty, useTabTitle } from '@/app/tabs/tabPage'
import AppBreadcrumb from '@/components/AppBreadcrumb.vue'
import DetailHeader, { type HeaderFact } from '@/components/DetailHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import OverviewFields, { type FieldDef } from '@/components/OverviewFields.vue'
import SaveBar from '@/components/SaveBar.vue'
import { useRoles } from '@/features/roles/api'
import { ADMINISTRATOR_ROLE_ID, effective } from '@/features/roles/catalog'
import type { Role } from '@/lib/api/types'
import { Perm } from '@/lib/auth/permissions'
import { useSession } from '@/lib/auth/session'
import { formatDateTime } from '@/lib/dates'
import { changeCount, changesOf, clearTab, discardTab, emptyDraft, isDirty, listOf, restoreTab, setList, pruneList } from '@/lib/detailDraft'
import { notify } from '@/lib/notify'
import { initials, inviteNote, relativeTime } from '@/lib/people'
import { useUrlState } from '@/lib/urlState'
import { useAccount } from '../api'
import { useAccountOverviewSave, useAccountRolesSave } from '../overviewSave'
import { statusSeverity } from '../status'
import { useAccountActions } from '../useAccountActions'

// Trang tài khoản: thẻ đầu trang (ai, trạng thái, hành động) rồi Overview | Roles | Activity
const props = defineProps<{ id: string }>()

const session = useSession()
const canManage = computed(() => session.can(Perm.AccountManage))
const { data: account } = useAccount(() => props.id)
useTabTitle(() => account.value?.name)
const roles = useRoles()
const actions = useAccountActions()
const self = computed(() => !!account.value && actions.isSelf(account.value))
// dải thông tin ở đầu trang: email, vai trò, lần đăng nhập gần nhất
const facts = computed<HeaderFact[]>(() => {
  const a = account.value
  if (!a) return []
  return [
    { label: 'Email', value: a.email },
    { label: 'Roles', value: a.roles.length ? a.roles.map((r) => r.name).join(', ') : 'No roles' },
    { label: 'Last sign-in', value: a.last_sign_in_at ? relativeTime(a.last_sign_in_at) : 'Never' },
  ]
})

type Section = 'overview' | 'roles'
const { state, update } = useUrlState(
  (q) => ({ tab: (q.tab === 'roles' ? 'roles' : 'overview') as Section }),
  (s) => ({ tab: s.tab === 'roles' ? s.tab : undefined }),
)
const label = (s: string) => s[0].toUpperCase() + s.slice(1)

// Bản nháp chung cho mọi tab: Overview (tên) và Roles (tick); một thanh lưu cho tab đang mở
const draft = reactive(emptyDraft())
// tab chia mục (một điều khiển phân đoạn với bộ lọc của danh sách)
const sections = computed<SegmentOption<Section>[]>(() => [
  { value: 'overview', label: 'Overview', dirty: changeCount(draft, 'overview') > 0 },
  { value: 'roles', label: 'Roles', count: account.value?.roles.length, dirty: changeCount(draft, 'roles') > 0 },
])
useTabDirty(() => isDirty(draft))
useLeaveGuard(() => isDirty(draft))
const fields: FieldDef[] = [
  { key: 'name', label: 'Display name', maxlength: 200 },
  { key: 'email', label: 'Email', lock: 'The sign-in address. Invite a new account to use another one.' },
]
const savedFields = computed(() => ({ name: account.value?.name ?? '', email: account.value?.email ?? '' }))

// Role: tick trong danh sách; bản nháp theo từng role so với role đã lưu
const saved = computed(() => account.value?.roles.map((r) => r.id) ?? [])
const allRoleIds = computed(() => (roles.data.value ?? []).map((r) => r.id))
const current = computed(() => listOf(draft, 'roles', saved.value, allRoleIds.value))
// danh sách đã lưu đổi: tick nay trùng thì không còn là thay đổi
watch(saved, (s) => pruneList(draft, 'roles', s))
function pick(id: string, on: boolean) {
  const next = on ? [...current.value, id] : current.value.filter((x) => x !== id)
  setList(draft, 'roles', saved.value, next)
}
const grantable = (r: Role) => r.permissions.every((p) => session.can(p))
// Không tự bỏ role Administrator của mình (API trả 409); báo trước khi lưu
const lockout = computed(() => self.value && saved.value.includes(ADMINISTRATOR_ROLE_ID) && !current.value.includes(ADMINISTRATOR_ROLE_ID))
const groups = computed(() => effective(roles.data.value ?? [], saved.value, current.value))

const saveOverview = useAccountOverviewSave()
const saveRoles = useAccountRolesSave()
const saving = ref(false)
const tabCount = computed(() => changeCount(draft, state.value.tab))
async function save() {
  const a = account.value
  if (!a) return
  const tab = state.value.tab
  saving.value = true
  try {
    const ok = tab === 'overview' ? await saveOverview(a, changesOf(draft, 'overview')) : await saveRoles(a, [...saved.value], current.value)
    if (ok) clearTab(draft, tab)
  } finally {
    saving.value = false
  }
}
function discard() {
  const tab = state.value.tab
  const removed = discardTab(draft, tab)
  notify.success('Changes discarded.', { undo: () => restoreTab(draft, tab, removed) })
}

// Hành động ít dùng: menu "More"
const more = ref<InstanceType<typeof Menu>>()
const moreItems = computed<MenuItem[]>(() => {
  const a = account.value
  if (!a) return []
  return [
    { label: 'Resend invitation', icon: 'pi pi-envelope', visible: a.status === 'invited', command: () => actions.resendInvitation(a) },
    { label: 'Send reset link', icon: 'pi pi-key', visible: a.status === 'active', command: () => actions.sendReset(a) },
    { label: 'Sign out everywhere', icon: 'pi pi-sign-out', visible: a.status === 'active', disabled: !a.active_sessions, command: () => actions.signOutEverywhere(a) },
  ]
})

const crumbs = computed(() => [{ label: 'Accounts', to: '/accounts' }, { label: account.value?.name ?? '…' }])
</script>

<template>
  <section v-if="account">
    <AppBreadcrumb :items="crumbs" />

    <DetailHeader :title="account.name" :facts="facts">
      <template #media>
        <Avatar :label="initials(account.name)" shape="circle" size="xlarge" :class="['avatar', { muted: account.status === 'disabled' }]" />
      </template>
      <template #tags>
        <Tag :value="label(account.status)" :severity="statusSeverity(account.status)" />
        <Tag v-if="self" value="You" severity="secondary" />
      </template>
      <template v-if="canManage" #actions>
        <Button v-if="account.status === 'disabled'" label="Enable" severity="secondary" outlined @click="actions.enable(account)" />
        <span v-else v-tooltip.top="self ? 'You can’t disable yourself' : undefined">
          <Button label="Disable" severity="secondary" outlined :disabled="self" @click="actions.disable(account)" />
        </span>
        <Button
          v-if="moreItems.some((i) => i.visible)"
          icon="pi pi-ellipsis-h"
          severity="secondary"
          outlined
          aria-label="More actions"
          aria-haspopup="menu"
          @click="(e: MouseEvent) => more?.toggle(e)"
        />
        <Menu ref="more" :model="moreItems" popup />
      </template>
    </DetailHeader>

    <SegmentedFilter :model-value="state.tab" :options="sections" label="Sections" class="section-tabs" @update:model-value="(v) => update({ tab: v })" />

    <div v-if="state.tab === 'overview'" class="grid">
      <Panel header="Profile">
        <OverviewFields :fields="fields" :saved="savedFields" :draft="draft" :readonly="!canManage" stacked />
        <p class="note">Linked member <Tag value="later" severity="secondary" class="later" />: linking to a person in the directory comes with the directory module.</p>
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
    </div>
    <SaveBar
      v-if="canManage && tabCount"
      :count="tabCount"
      :saving="saving"
      :blocked="state.tab === 'roles' && lockout"
      @save="save"
      @discard="discard"
    />
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
