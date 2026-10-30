<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {HardDrive,FolderTree,Users,RefreshCw,Plus,X,Gauge} from 'lucide-vue-next'
import {useHost,createApiClient,ApiError} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
import {fromGB,toGB,type Home,type Share,type Volume} from './api'

// Storage Manager (com.minhanos.storage): volumes, shared folders and per-user soft quota.
// Writes need storage.manage and a reauthentication; the server enforces both.
const props=withDefaults(defineProps<{active?:boolean;apiBase?:string}>(),{active:true,apiBase:'/api'})
const host=useHost(),release=host.useCatalog('storage',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t('storage.'+key,params)}
const api=createApiClient(props.apiBase)
type Tab='volumes'|'shares'|'homes'
const tab=ref<Tab>('shares'),loading=ref(false),busy=ref(false),error=ref(''),notice=ref('')
const volumes=ref<Volume[]>([]),shares=ref<Share[]>([]),homes=ref<Home[]>([]),agentError=ref(false)
type Dialog={kind:'share'|'share-quota'|'home-quota'|'measure';title:string;target?:Share|Home}
const dialog=ref<Dialog|null>(null),form=reactive({key:'',name:'',volume:'',quota:'',password:''})
const managedVolumes=computed(()=>volumes.value.filter(v=>v.managed&&v.online))

function size(bytes:number|null|undefined){
 if(bytes==null)return t('unlimited')
 const units=['B','KB','MB','GB','TB'];let v=bytes,i=0
 while(v>=1024&&i<units.length-1){v/=1024;i++}
 return host.i18n.formatNumber(v,{maximumFractionDigits:i?1:0})+' '+units[i]
}
function percent(used:number,limit:number|null){return limit?Math.min(100,Math.round(used*100/Math.max(limit,1))):0}
function message(e:unknown){if(e instanceof ApiError){const key='errors.'+e.code;return ('storage.'+key in catalogs['vi-VN'])?t(key):host.errorMessage(e)}return t('errors.network')}

async function load(){
 loading.value=true;error.value=''
 try{
  const [v,s,h]=await Promise.all([api.request<{items:Volume[];agentError:boolean}>('/storage/volumes'),api.request<{items:Share[]}>('/storage/shares'),api.request<{items:Home[]}>('/storage/homes')])
  volumes.value=v.items;agentError.value=v.agentError;shares.value=s.items;homes.value=h.items
 }catch(e){error.value=message(e)}finally{loading.value=false}
}
async function init(){
 try{const s=await api.request<{csrfToken:string}>('/session');api.setCSRF(s.csrfToken);await load()}catch(e){error.value=message(e)}
}
function open(kind:Dialog['kind'],target?:Share|Home){
 if(busy.value)return
 Object.assign(form,{key:'',name:'',volume:managedVolumes.value[0]?.name??'',quota:'',password:''})
 if(target)form.quota=toGB(target.usage.limitBytes)
 error.value=''
 dialog.value={kind,target,title:t(kind==='share'?'shares.new':kind==='measure'?'homes.measure':'quota.edit')}
}
function close(){if(!busy.value){dialog.value=null;form.password=''}}
async function submit(){
 const d=dialog.value;if(!d||busy.value)return
 const limitBytes=d.kind==='measure'?null:fromGB(form.quota)
 if(limitBytes===undefined){error.value=t('errors.quota');return}
 busy.value=true;error.value=''
 try{
  await api.request('/v1/identity/reauthenticate',{method:'POST',body:JSON.stringify({password:form.password})})
  form.password=''
  if(d.kind==='share')await api.request('/storage/shares',{method:'POST',body:JSON.stringify({key:form.key,name:form.name,volume:form.volume,limitBytes})})
  else if(d.kind==='share-quota'){const s=d.target as Share;await api.request('/storage/shares/'+s.id+'/quota',{method:'PUT',body:JSON.stringify({version:s.version,limitBytes})})}
  else if(d.kind==='measure'){const h=d.target as Home;await api.request('/storage/homes/'+h.userId+'/measure',{method:'POST',body:'{}'})}
  else{const h=d.target as Home;await api.request('/storage/homes/'+h.userId+'/quota',{method:'PUT',body:JSON.stringify({limitBytes})})}
  notice.value=t(d.kind==='measure'?'homes.measureQueued':'saved');dialog.value=null;await load()
 }catch(e){error.value=message(e)}finally{form.password='';busy.value=false}
}
function canClose(){return !busy.value&&(!dialog.value||globalThis.confirm(t('confirm.discard')))}
defineExpose({canClose})
watch(()=>props.active,a=>{if(!a)form.password=''})
onMounted(init)
</script>

<template>
 <section class="mn-storage" :aria-label="t('title')">
  <aside class="sm-sidebar">
   <h1><HardDrive :size="20"/>{{t('title')}}</h1>
   <nav :aria-label="t('title')">
    <button v-for="item in ([['shares',FolderTree],['homes',Users],['volumes',HardDrive]] as const)" :key="item[0]" :class="{selected:tab===item[0]}" :aria-current="tab===item[0]?'page':undefined" :disabled="busy" @click="tab=item[0];notice=''"><component :is="item[1]" :size="18"/>{{t('nav.'+item[0])}}</button>
   </nav>
   <p class="sm-muted">{{t('softQuotaHint')}}</p>
  </aside>
  <main class="sm-main" :aria-busy="loading||busy">
   <header class="sm-header"><div><h2>{{t('nav.'+tab)}}</h2><p class="sm-muted">{{t('intro.'+tab)}}</p></div><div class="sm-actions"><button v-if="tab==='shares'" class="sm-primary" :disabled="busy||!managedVolumes.length" @click="open('share')"><Plus :size="17"/>{{t('shares.new')}}</button><button :aria-label="t('refresh')" :title="t('refresh')" :disabled="loading||busy" @click="load"><RefreshCw :size="17"/></button></div></header>
   <p v-if="notice" class="sm-notice" role="status">{{notice}}</p>
   <p v-if="error&&!dialog" class="sm-error" role="alert">{{error}}</p>
   <p v-if="agentError" class="sm-error" role="alert">{{t('errors.agentOffline')}}</p>

   <div v-if="tab==='shares'" class="sm-list">
    <article v-for="s in shares" :key="s.id" class="sm-card">
     <div class="sm-row"><span class="sm-title"><FolderTree :size="20"/><span><strong>{{s.kind==='homes'?t('shares.homesName'):s.name}}</strong><small>{{s.key}} · {{t('shares.onVolume',{volume:s.volume})}}</small></span></span><button :disabled="busy" @click="open('share-quota',s)"><Gauge :size="16"/>{{t('quota.edit')}}</button></div>
     <div class="sm-bar" role="progressbar" :aria-valuenow="percent(s.usage.usedBytes,s.usage.limitBytes)" aria-valuemin="0" aria-valuemax="100"><i :style="{width:percent(s.usage.usedBytes,s.usage.limitBytes)+'%'}"></i></div>
     <small>{{t('quota.usedOf',{used:size(s.usage.usedBytes),limit:size(s.usage.limitBytes)})}}<template v-if="s.physicalBytes!=null"> · {{t('quota.physical',{size:size(s.physicalBytes)})}}</template></small>
    </article>
    <p v-if="!shares.length&&!loading" class="sm-muted">{{t('empty')}}</p>
   </div>

   <div v-if="tab==='homes'" class="sm-table-wrap"><table><thead><tr><th>{{t('homes.user')}}</th><th>{{t('homes.usage')}}</th><th><span class="sm-sr">{{t('homes.actions')}}</span></th></tr></thead><tbody>
    <tr v-for="h in homes" :key="h.userId"><td><strong>{{h.displayName}}</strong><small>@{{h.username}}</small></td><td><div class="sm-bar"><i :style="{width:percent(h.usage.usedBytes,h.usage.limitBytes)+'%'}"></i></div><small>{{h.usage.measured?t('quota.usedOf',{used:size(h.usage.usedBytes),limit:size(h.usage.limitBytes)}):t('homes.notMeasured')}}</small></td><td class="sm-actions"><button :disabled="busy" @click="open('home-quota',h)">{{t('quota.edit')}}</button><button :disabled="busy" @click="open('measure',h)">{{t('homes.measure')}}</button></td></tr>
   </tbody></table></div>

   <div v-if="tab==='volumes'" class="sm-list">
    <article v-for="v in volumes" :key="v.name" class="sm-card"><div class="sm-row"><span class="sm-title"><HardDrive :size="20"/><span><strong>{{v.name}}</strong><small>{{t('kinds.'+v.kind)}}</small></span></span><span class="sm-pill" :class="{off:!v.online}">{{t(v.online?'volumes.online':'volumes.offline')}}</span></div><small>{{t(v.managed?'volumes.managed':'volumes.plain')}} · {{t('volumes.snapshots')}}: {{t(v.snapshots?'yes':'no')}}</small></article>
   </div>
  </main>

  <div v-if="dialog&&active" class="sm-overlay" @click.self="close"><form class="sm-dialog" role="dialog" aria-modal="true" aria-labelledby="sm-dialog-title" @submit.prevent="submit">
   <div class="sm-row"><h2 id="sm-dialog-title">{{dialog.title}}</h2><button type="button" :aria-label="t('close')" :disabled="busy" @click="close"><X :size="18"/></button></div>
   <p v-if="error" class="sm-error" role="alert">{{error}}</p>
   <fieldset :disabled="busy">
    <template v-if="dialog.kind==='share'">
     <label>{{t('shares.name')}}<input v-model="form.name" required maxlength="100"></label>
     <label>{{t('shares.key')}}<input v-model="form.key" required pattern="[a-z0-9][a-z0-9\-]{0,62}" maxlength="63"><small>{{t('shares.keyHint')}}</small></label>
     <label>{{t('shares.volume')}}<select v-model="form.volume" required><option v-for="v in managedVolumes" :key="v.name" :value="v.name">{{v.name}} ({{t('kinds.'+v.kind)}})</option></select></label>
    </template>
    <p v-else class="sm-muted">{{dialog.kind==='share-quota'?(dialog.target as Share).name:(dialog.target as Home).displayName}}</p>
    <label v-if="dialog.kind!=='measure'">{{t('quota.limitGB')}}<input v-model="form.quota" inputmode="decimal" :placeholder="t('unlimited')"><small>{{t('quota.limitHint')}}</small></label>
    <label>{{t('confirm.password')}}<input v-model="form.password" type="password" autocomplete="current-password" required maxlength="256"><small>{{t('confirm.reauth')}}</small></label>
   </fieldset>
   <footer class="sm-actions"><button type="button" :disabled="busy" @click="close">{{t('cancel')}}</button><button class="sm-primary" :disabled="busy">{{t(busy?'working':'save')}}</button></footer>
  </form></div>
 </section>
</template>
