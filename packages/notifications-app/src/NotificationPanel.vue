<script setup lang="ts">
import {onBeforeUnmount,onMounted} from 'vue'
import {X,ArrowRight,CheckCheck} from 'lucide-vue-next'
import {useHost} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
import {errorText,type Inbox} from './inbox'
import NoticeRow from './NoticeRow.vue'

// Bell panel rendered by the Shell: latest notices, mark read, open the full Notification Center.
const props=defineProps<{inbox:Inbox}>()
defineEmits<{open:[];close:[]}>()
const host=useHost(),release=host.useCatalog('notifications',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t('notifications.'+key,params)}
const fullT=(key:string,params?:Record<string,string|number>)=>host.i18n.t(key,params)
const {items,unread,loading,error}=props.inbox
onMounted(()=>props.inbox.load({limit:8}))
</script>

<template>
 <div class="mn-notify-panel">
  <header><strong id="nc-panel-title">{{t('title')}}</strong><span class="nc-muted">{{unread?t('unreadCount',{count:unread}):t('allRead')}}</span><button class="nc-close" :aria-label="t('close')" @click="$emit('close')"><X :size="18"/></button></header>
  <p v-if="error" class="nc-error" role="alert">{{errorText(fullT,host,error)}}</p>
  <div class="nc-list" :aria-busy="loading">
   <NoticeRow v-for="n in items.slice(0,8)" :key="n.id" :notice="n" compact @read="id=>inbox.markRead([id])"/>
   <p v-if="!items.length&&!loading&&!error" class="nc-muted nc-empty-line">{{t('empty')}}</p>
  </div>
  <footer>
   <button :disabled="!unread" @click="inbox.markAll"><CheckCheck :size="16"/>{{t('markAll')}}</button>
   <button @click="$emit('open')">{{t('openCenter')}}<ArrowRight :size="16"/></button>
  </footer>
 </div>
</template>
