<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,ref} from 'vue'
import {useHost,ApiError} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
import {createFilesClient} from './client'

// Central Settings panel owned by Files (ADR 0012): how much the Files activity log records. System Admin only;
// the change itself is always audited by the Files service.
const props=withDefaults(defineProps<{active?:boolean;apiBase?:string}>(),{active:true,apiBase:'/api/drive'})
const host=useHost(),release=host.useCatalog('files',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t('files.activity.'+key,params)}
const client=createFilesClient(props.apiBase)
const levels=['minimal','writes','access','full'] as const
type Level=typeof levels[number]
const level=ref<Level>('minimal'),saved=ref<Level>('minimal'),version=ref(0),retention=ref(0),busy=ref(false),error=ref(''),notice=ref('')
const dirty=computed(()=>level.value!==saved.value)
function message(e:unknown){const key='files.errors.'+(e instanceof ApiError?e.code:'');return key in catalogs['vi-VN']?host.i18n.t(key):e instanceof ApiError?host.errorMessage(e):t('network')}
async function load(){const v=await client.request<{level:Level;version:number;retentionDays?:number}>('/activity/settings');level.value=saved.value=v.level;version.value=v.version;retention.value=v.retentionDays??0}
onMounted(async()=>{try{const s=await client.request<{csrfToken:string}>('/session');client.setCSRF(s.csrfToken);await load()}catch(e){error.value=message(e)}})
async function save(){
 busy.value=true;error.value='';notice.value=''
 try{const v=await client.request<{level:Level;version:number}>('/activity/settings',{method:'PUT',headers:{'If-Match':'"'+version.value+'"'},body:JSON.stringify({level:level.value})});saved.value=v.level;version.value=v.version;notice.value=t('saved')}
 catch(e){error.value=message(e);if(e instanceof ApiError&&e.code==='VERSION_CONFLICT')try{await load()}catch{}}
 finally{busy.value=false}
}
function canClose(){return !dirty.value||globalThis.confirm(t('discard'))}
defineExpose({canClose})
void props
</script>

<template>
 <form class="st-page" @submit.prevent="save">
  <h2>{{t('title')}}</h2>
  <p class="st-muted">{{t('hint')}}</p>
  <fieldset class="st-rows fa-levels" :disabled="busy||!version">
   <legend class="fa-sr">{{t('title')}}</legend>
   <label v-for="l in levels" :key="l" class="st-row fa-level">
    <input v-model="level" type="radio" name="files-activity-level" :value="l">
    <span><strong>{{t('levels.'+l)}}</strong><small>{{t('describe.'+l)}}</small></span>
   </label>
  </fieldset>
  <p class="st-muted">{{t('privacy')}}<template v-if="retention"> {{t('retention',{days:retention})}}</template></p>
  <p v-if="error" class="st-error" role="alert">{{error}}</p>
  <p v-if="notice" class="st-muted" role="status">{{notice}}</p>
  <div><button class="st-save" :disabled="busy||!dirty">{{t(busy?'working':'save')}}</button></div>
 </form>
</template>

<style scoped>
.fa-levels{margin:0;padding:0;min-width:0}
.fa-sr{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)}
.fa-level{justify-content:flex-start;align-items:flex-start;cursor:pointer}
.fa-level input{margin-top:3px;min-height:auto}
.fa-level span{display:flex;flex-direction:column;gap:2px;min-width:0;flex:1}
.fa-level small{color:var(--mn-color-text-secondary);font-size:13px;line-height:1.4}
</style>
