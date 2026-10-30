<script setup lang="ts">
import {computed} from 'vue'
import {Info,TriangleAlert,ShieldAlert,Check} from 'lucide-vue-next'
import {useHost} from '@minhanos/app-sdk'
import {noticeText,sourceName,type Notice} from './inbox'

// One notice: severity icon, localized title/body, source app and time. Shared by the app and the bell panel.
const props=defineProps<{notice:Notice;compact?:boolean}>()
defineEmits<{read:[id:string]}>()
const host=useHost()
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t(key,params)}
const text=computed(()=>noticeText(t,props.notice))
const icon=computed(()=>({info:Info,warning:TriangleAlert,critical:ShieldAlert})[props.notice.severity])
const time=computed(()=>(host.profile.value.locale,host.i18n.formatDate(props.notice.createdAt,{day:'2-digit',month:'2-digit',hour:'2-digit',minute:'2-digit'})))
</script>

<template>
 <article class="nc-notice" :class="[notice.severity,{unread:!notice.readAt,compact}]">
  <span class="nc-icon" aria-hidden="true"><component :is="icon" :size="18"/></span>
  <div class="nc-text">
   <strong>{{text.title}}</strong>
   <p>{{text.body}}</p>
   <small>{{sourceName(t,notice.source)}} · <time :datetime="notice.createdAt">{{time}}</time><template v-if="notice.mandatory"> · {{t('notifications.mandatory')}}</template></small>
  </div>
  <button v-if="!notice.readAt" class="nc-read" :aria-label="t('notifications.markRead')" :title="t('notifications.markRead')" @click="$emit('read',notice.id)"><Check :size="16"/></button>
  <span v-if="!notice.readAt" class="nc-sr">{{t('notifications.unread')}}</span>
 </article>
</template>
