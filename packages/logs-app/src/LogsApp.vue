<script setup lang="ts">
import {onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {ScrollText,RefreshCw,ChevronDown,ChevronRight} from 'lucide-vue-next'
import {useHost,ApiError} from '@minhanos/app-sdk'
import {createLogsClient} from '@minhanos/api-client/logs'
import {createIdentityClient} from '@minhanos/api-client/identity'
import type {LogEvent,LogSource,IdentityOrganization} from '@minhanos/shared-types'
import catalogs from '../locales/catalogs.json'

// Log Center (com.minhanos.logs), minimal v1: read-only audit events of every module, newest first.
// Needs logs.audit.read (System Admin); the server enforces it. Actions are stable codes, not prose.
const props=withDefaults(defineProps<{active?:boolean;apiBase?:string}>(),{active:true,apiBase:'/api'})
const host=useHost(),release=host.useCatalog('logs',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t('logs.'+key,params)}
const known=(key:string)=>'logs.'+key in catalogs['vi-VN']
const logs=createLogsClient(props.apiBase+'/v1/logs')
// Organization admins see only their organizations' events (ADR 0008); the filter narrows to one of them.
const identity=createIdentityClient(props.apiBase+'/v1/identity'),orgs=ref<IdentityOrganization[]>([])
const orgName=(id:string|null|undefined)=>orgs.value.find(o=>o.id===id)?.name??id??''
const filter=reactive({source:'',outcome:'',organization:''})
const events=ref<LogEvent[]>([]),sources=ref<LogSource[]>([]),next=ref<string|null>(null),loading=ref(false),error=ref(''),open=ref<string|null>(null)

function message(e:unknown){if(e instanceof ApiError){return known('errors.'+e.code)?t('errors.'+e.code):host.errorMessage(e)}return t('errors.network')}
const when=(value:string)=>(host.profile.value.locale,host.i18n.formatDate(value,{day:'2-digit',month:'2-digit',year:'numeric',hour:'2-digit',minute:'2-digit',second:'2-digit'}))
const action=(code:string)=>known('actions.'+code)?t('actions.'+code):code
const detail=(value:unknown)=>typeof value==='string'||typeof value==='number'||typeof value==='boolean'?String(value):JSON.stringify(value)

async function load(more=false){
 if(loading.value)return
 loading.value=true;error.value=''
 try{
  const page=await logs.logsListEvents({source:filter.source||undefined,outcome:filter.outcome||undefined,organization:filter.organization||undefined,limit:50,cursor:more?next.value??undefined:undefined})
  events.value=more?[...events.value,...page.items]:page.items;next.value=page.nextCursor
 }catch(e){error.value=message(e)}finally{loading.value=false}
}
watch(filter,()=>load())
watch(()=>props.active,a=>{if(a&&!events.value.length)void load()})
onMounted(async()=>{await load();try{orgs.value=(await identity.identityListOrganizations({limit:100})).items.filter(o=>o.role==='root'||o.role==='admin')}catch{/* the filter is optional */}try{sources.value=(await logs.logsListSources()).items}catch{/* retention line is optional */}})
defineExpose({canClose:()=>true})
</script>

<template>
 <section class="mn-logs" :aria-label="t('title')">
  <header class="lc-header">
   <div><h1><ScrollText :size="20"/>{{t('title')}}</h1><p class="lc-muted">{{t('intro')}}</p></div>
   <button :aria-label="t('refresh')" :title="t('refresh')" :disabled="loading" @click="load()"><RefreshCw :size="17"/></button>
  </header>
  <div class="lc-filters">
   <label>{{t('filters.source')}}<select v-model="filter.source"><option value="">{{t('filters.all')}}</option><option v-for="s in ['identity','storage','files']" :key="s" :value="s">{{t('sources.'+s)}}</option></select></label>
   <label>{{t('filters.outcome')}}<select v-model="filter.outcome"><option value="">{{t('filters.all')}}</option><option v-for="o in ['success','denied','failure']" :key="o" :value="o">{{t('outcomes.'+o)}}</option></select></label>
   <label v-if="orgs.length>1">{{t('filters.organization')}}<select v-model="filter.organization"><option value="">{{t('filters.all')}}</option><option v-for="o in orgs" :key="o.id" :value="o.id">{{o.name}}</option></select></label>
  </div>
  <p v-if="error" class="lc-error" role="alert">{{error}}</p>
  <div class="lc-table" :aria-busy="loading">
   <table>
    <thead><tr><th>{{t('columns.time')}}</th><th>{{t('columns.source')}}</th><th>{{t('columns.action')}}</th><th>{{t('columns.actor')}}</th><th>{{t('columns.target')}}</th><th>{{t('columns.outcome')}}</th></tr></thead>
    <tbody>
     <template v-for="e in events" :key="e.id">
      <tr>
       <td><button class="lc-expand" :aria-expanded="open===e.id" :aria-label="t('details')" @click="open=open===e.id?null:e.id"><component :is="open===e.id?ChevronDown:ChevronRight" :size="15"/></button><time :datetime="e.occurredAt">{{when(e.occurredAt)}}</time></td>
       <td>{{t('sources.'+e.source)}}</td>
       <td><code>{{action(e.action)}}</code></td>
       <td>{{e.actor?(e.actor.username||e.actor.id):t('system')}}</td>
       <td class="lc-target">{{e.targetName??e.target??'—'}}</td>
       <td><span class="lc-pill" :class="e.outcome">{{t('outcomes.'+e.outcome)}}</span></td>
      </tr>
      <tr v-if="open===e.id" class="lc-detail"><td colspan="6"><dl>
       <dt>{{t('requestId')}}</dt><dd><code>{{e.requestId}}</code></dd>
       <template v-if="e.organizationId"><dt>{{t('organization')}}</dt><dd>{{orgName(e.organizationId)}}</dd></template>
       <template v-if="e.correlationId"><dt>{{t('correlationId')}}</dt><dd><code>{{e.correlationId}}</code></dd></template>
       <template v-if="e.target&&e.targetName"><dt>{{t('columns.target')}}</dt><dd><code>{{e.target}}</code></dd></template>
       <template v-for="(value,key) in e.details" :key="key"><dt>{{key}}</dt><dd><code>{{detail(value)}}</code></dd></template>
      </dl></td></tr>
     </template>
    </tbody>
   </table>
   <p v-if="!events.length&&!loading&&!error" class="lc-muted lc-empty">{{t('empty')}}</p>
  </div>
  <button v-if="next" class="lc-more" :disabled="loading" @click="load(true)">{{t(loading?'loading':'more')}}</button>
  <p v-if="sources.length" class="lc-muted lc-foot">{{t('retention')}}: <template v-for="(s,i) in sources" :key="s.id">{{i?' · ':''}}{{t('sources.'+s.id)}} {{s.retentionDays?t('retentionDays',{days:s.retentionDays}):t('retentionNone')}}</template></p>
 </section>
</template>
