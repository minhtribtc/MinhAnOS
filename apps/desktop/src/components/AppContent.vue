<script setup lang="ts">
import { computed, ref } from 'vue'
import type { AppDefinition } from '../api'
import { api } from '../api'
import { useDesktop } from '../store'
import FilesApp from '@minhanos/files-app'
import AccountsApp from '@minhanos/accounts-app'
import SettingsApp from '@minhanos/settings-app'
import StorageApp from '@minhanos/storage-app'
import NotificationsApp from '@minhanos/notifications-app'
import LogsApp from '@minhanos/logs-app'
import NotesApp from '@minhanos/notes-app'
import { installedSettingsProviders } from '../settings'
// The Shell hosts peer app packages only (app standard row 1); it has no built-in app screens of its own.
const instance=ref<{canClose:()=>boolean}|null>(null)
defineExpose({canClose:()=>instance.value?.canClose() ?? true})
const props = defineProps<{ app: AppDefinition; foreground?:boolean }>()
const desktop = useDesktop()
const active = computed(() => props.foreground!==false)
// Effective permissions only decide which Settings entries are listed; each panel's API still enforces access.
const permissions = ref<string[]|null>(null)
const settingsProviders = computed(() => installedSettingsProviders(desktop.apps.map(a => a.id)))
if (props.app.id === 'settings') void api<{ permissions: string[] }>('/v1/identity/me/access').then(r => { permissions.value = r.permissions }, () => { permissions.value = [] })
</script>

<template>
  <SettingsApp v-if="app.id === 'settings'" ref="instance" :providers="settingsProviders" :permissions="permissions" :active="active" :target="desktop.settingsTarget" />
  <AccountsApp v-else-if="app.id === 'com.minhanos.account-center'" ref="instance" embedded :active="active" />
  <StorageApp v-else-if="app.id === 'com.minhanos.storage'" ref="instance" :active="active" />
  <NotificationsApp v-else-if="app.id === 'com.minhanos.notifications'" ref="instance" :active="active" />
  <LogsApp v-else-if="app.id === 'com.minhanos.logs'" ref="instance" :active="active" />
  <FilesApp v-else-if="app.id === 'files'" ref="instance" :active="active" embedded api-base="/api/drive" />
  <NotesApp v-else-if="app.id === 'com.minhanos.notes'" ref="instance" v-model="desktop.preferences.notes" :display-name="desktop.user?.displayName" :saving="desktop.saving" :save-failed="!!desktop.saveError" :active="active" @update:model-value="desktop.saveSoon()" @retry="desktop.persist" />
</template>
