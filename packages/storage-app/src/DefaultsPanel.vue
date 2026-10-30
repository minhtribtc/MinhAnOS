<script setup lang="ts">
import {onBeforeUnmount,onMounted,ref} from 'vue'
import {useHost,createApiClient,ApiError} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
import {fromGB,toGB} from './api'

// Settings panel owned by Storage Manager (hybrid rule): only configuration, no resource management.
const props=withDefaults(defineProps<{active?:boolean}>(),{active:true})
const host=useHost(),release=host.useCatalog('storage',catalogs)
onBeforeUnmount(release)
const t=(key:string)=>{host.profile.value.locale;return host.i18n.t('storage.'+key)}
const api=createApiClient('/api')
const quota=ref(''),saved=ref(''),password=ref(''),busy=ref(false),error=ref(''),notice=ref('')
onMounted(async()=>{try{const s=await api.request<{csrfToken:string}>('/session');api.setCSRF(s.csrfToken);const v=await api.request<{defaultHomeLimitBytes:number|null}>('/storage/settings');quota.value=saved.value=toGB(v.defaultHomeLimitBytes)}catch(e){error.value=e instanceof ApiError?host.errorMessage(e):t('errors.network')}})
async function save(){
 const limit=fromGB(quota.value)
 if(limit===undefined){error.value=t('errors.quota');return}
 busy.value=true;error.value='';notice.value=''
 try{await api.request('/v1/identity/reauthenticate',{method:'POST',body:JSON.stringify({password:password.value})});await api.request('/storage/settings',{method:'PUT',body:JSON.stringify({defaultHomeLimitBytes:limit})});saved.value=quota.value;notice.value=t('saved')}
 catch(e){error.value=e instanceof ApiError?(('storage.errors.'+e.code) in catalogs['vi-VN']?t('errors.'+e.code):host.errorMessage(e)):t('errors.network')}
 finally{password.value='';busy.value=false}
}
function canClose(){return quota.value===saved.value||globalThis.confirm(t('confirm.discard'))}
defineExpose({canClose})
void props
</script>

<template>
 <form class="st-page" @submit.prevent="save">
  <h2>{{t('settings.defaults')}}</h2>
  <p class="st-muted">{{t('settings.defaultHint')}}</p>
  <div class="st-rows">
   <label class="st-row"><span>{{t('settings.defaultQuota')}}</span><input v-model="quota" inputmode="decimal" :placeholder="t('unlimited')" :disabled="busy"></label>
   <label class="st-row"><span>{{t('confirm.password')}}</span><input v-model="password" type="password" autocomplete="current-password" required maxlength="256" :disabled="busy"></label>
  </div>
  <p v-if="error" class="st-error" role="alert">{{error}}</p>
  <p v-if="notice" class="st-muted" role="status">{{notice}}</p>
  <div><button class="st-save" :disabled="busy||quota===saved">{{t(busy?'working':'save')}}</button></div>
 </form>
</template>
