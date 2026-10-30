<script setup lang="ts">
import {onBeforeUnmount,onMounted,ref,watch} from 'vue'
import {Bell,RefreshCw,CheckCheck} from 'lucide-vue-next'
import {useHost} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
import {createInbox,errorText} from './inbox'
import NoticeRow from './NoticeRow.vue'

// Notification Center (com.minhanos.notifications), minimal v1: the signed-in user's inbox.
// Notices come from backend modules only (security notices from Account Center today).
const props=withDefaults(defineProps<{active?:boolean;apiBase?:string}>(),{active:true,apiBase:'/api'})
const host=useHost(),release=host.useCatalog('notifications',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t('notifications.'+key,params)}
const fullT=(key:string,params?:Record<string,string|number>)=>host.i18n.t(key,params)
const inbox=createInbox(props.apiBase),{items,unread,next,loading,error}=inbox
const filter=ref<'all'|'unread'>('all')
const load=(more=false)=>inbox.load({unreadOnly:filter.value==='unread',more})
watch(filter,()=>load())
// Refresh when the window comes back to the foreground; no background polling inside the app.
watch(()=>props.active,a=>{if(a)void load()})
onMounted(()=>load())
defineExpose({canClose:()=>true})
</script>

<template>
 <section class="mn-notifications" :aria-label="t('title')">
  <header class="nc-header">
   <div><h1><Bell :size="20"/>{{t('title')}}</h1><p class="nc-muted">{{t('intro')}}</p></div>
   <div class="nc-actions">
    <button :disabled="loading||!unread" @click="inbox.markAll"><CheckCheck :size="17"/>{{t('markAll')}}</button>
    <button :aria-label="t('refresh')" :title="t('refresh')" :disabled="loading" @click="load()"><RefreshCw :size="17"/></button>
   </div>
  </header>
  <div class="nc-tabs" role="tablist" :aria-label="t('filter')">
   <button v-for="f in (['all','unread'] as const)" :key="f" role="tab" :aria-selected="filter===f" :class="{selected:filter===f}" @click="filter=f">{{t('filters.'+f)}}<b v-if="f==='unread'&&unread">{{unread}}</b></button>
  </div>
  <p v-if="error" class="nc-error" role="alert">{{errorText(fullT,host,error)}}</p>
  <div class="nc-list" :aria-busy="loading">
   <NoticeRow v-for="n in items" :key="n.id" :notice="n" @read="id=>inbox.markRead([id])"/>
   <div v-if="!items.length&&!loading&&!error" class="nc-empty"><Bell :size="32"/><p>{{t(filter==='unread'?'emptyUnread':'empty')}}</p></div>
   <button v-if="next" class="nc-more" :disabled="loading" @click="load(true)">{{t(loading?'loading':'more')}}</button>
  </div>
  <p class="nc-muted nc-foot">{{t('retention')}}</p>
 </section>
</template>
