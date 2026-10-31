<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,reactive,ref,watch} from 'vue'
import {HardDrive,FolderTree,Users,RefreshCw,Plus,X,Gauge,ShieldCheck,Trash2,Info,User,FolderLock,Ruler} from 'lucide-vue-next'
import {useHost,createApiClient,createIdempotencyKey,ApiError} from '@minhanos/app-sdk'
import {createStorageClient} from '@minhanos/api-client/storage'
import catalogs from '../locales/catalogs.json'
import {fromGB,toGB,type AccessEntry,type Home,type QuotaFolder,type Share,type Subject,type Volume} from './api'

// Storage Manager (com.minhanos.storage): volumes, shared folders and per-user soft quota.
// Writes need storage.manage and a reauthentication; the server enforces both.
const props=withDefaults(defineProps<{active?:boolean;apiBase?:string}>(),{active:true,apiBase:'/api'})
const host=useHost(),release=host.useCatalog('storage',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t('storage.'+key,params)}
// Session/reauthentication use the host API; storage calls use the client generated from OpenAPI.
const api=createApiClient(props.apiBase),storage=createStorageClient(props.apiBase+'/v1/storage')
type Tab='volumes'|'shares'|'homes'
const tab=ref<Tab>('shares'),loading=ref(false),busy=ref(false),error=ref(''),notice=ref('')
const volumes=ref<Volume[]>([]),shares=ref<Share[]>([]),homes=ref<Home[]>([]),agentError=ref(false)
type Dialog={kind:'share'|'share-quota'|'home-quota'|'measure'|'access'|'share-measure'|'folder-new'|'folder-quota'|'folder-remove';version?:number;title:string;target?:Share|Home|QuotaFolder}
const dialog=ref<Dialog|null>(null),form=reactive({key:'',name:'',volume:'',quota:'',password:'',hostsHomes:false,path:''})
const managedVolumes=computed(()=>volumes.value.filter(v=>v.managed&&v.online))
// First-run rule: until one share hosts homes, Files has no personal space.
const hasHomes=computed(()=>shares.value.some(s=>s.kind==='homes'))
// Permission editor of one share (entries are replaced as a whole, If-Match on the share version).
const entries=ref<AccessEntry[]>([]),subjects=ref<Subject[]>([]),pick=ref('')
const available=computed(()=>subjects.value.filter(s=>!entries.value.some(e=>e.subjectType===s.type&&e.subjectId===s.id)))
// Quota folders (Q3) of the share whose panel is open; loaded on demand.
const openShare=ref<string|null>(null),folders=ref<QuotaFolder[]>([])
async function loadFolders(){if(!openShare.value)return;try{folders.value=(await storage.storageListQuotaFolders({shareId:openShare.value})).items}catch(e){error.value=message(e)}}
async function toggleFolders(s:Share){openShare.value=openShare.value===s.id?null:s.id;folders.value=[];await loadFolders()}

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
  const [v,s,h]=await Promise.all([storage.storageListVolumes(),storage.storageListShares(),storage.storageListHomes()])
  volumes.value=v.items;agentError.value=v.agentError;shares.value=s.items;homes.value=h.items
 }catch(e){error.value=message(e)}finally{loading.value=false}
}
async function init(){
 try{const s=await api.request<{csrfToken:string}>('/session');api.setCSRF(s.csrfToken);storage.setCSRF(s.csrfToken);await load()}catch(e){error.value=message(e)}
}
async function open(kind:Dialog['kind'],target?:Share|Home|QuotaFolder){
 if(busy.value)return
 Object.assign(form,{key:'',name:'',volume:managedVolumes.value[0]?.name??'',quota:'',password:'',hostsHomes:false,path:''})
 if(target&&kind!=='folder-new')form.quota=toGB(target.usage.limitBytes)
 error.value=''
 if(kind==='access'){
  const s=target as Share
  busy.value=true
  try{
   const [list,people]=await Promise.all([storage.storageGetShareAccess({shareId:s.id}),storage.storageListSubjects()])
   entries.value=list.entries;subjects.value=people.items;pick.value=''
   dialog.value={kind,target,version:list.version,title:t('access.title',{name:s.name})}
  }catch(e){error.value=message(e)}finally{busy.value=false}
  return
 }
 const titles:Partial<Record<Dialog['kind'],string>>={share:'shares.new',measure:'homes.measure','share-measure':'homes.measure','folder-new':'folders.new','folder-remove':'folders.remove'}
 dialog.value={kind,target,title:t(titles[kind]??'quota.edit')}
}
function addEntry(){
 const s=available.value.find(x=>x.type+':'+x.id===pick.value)
 if(!s||entries.value.length>=200)return
 entries.value=[...entries.value,{subjectType:s.type,subjectId:s.id,name:s.name,access:'read'}];pick.value=''
}
function removeEntry(entry:AccessEntry){entries.value=entries.value.filter(e=>e!==entry)}
function close(){if(!busy.value){dialog.value=null;form.password=''}}
async function submit(){
 const d=dialog.value;if(!d||busy.value)return
 const limitBytes=['measure','access','share-measure','folder-remove'].includes(d.kind)?null:fromGB(form.quota)
 if(limitBytes===undefined){error.value=t('errors.quota');return}
 busy.value=true;error.value=''
 try{
  await api.request('/v1/identity/reauthenticate',{method:'POST',body:JSON.stringify({password:form.password})})
  form.password=''
  if(d.kind==='share')await storage.storageCreateShare({idempotencyKey:createIdempotencyKey(),body:{key:form.key,name:form.name,volume:form.volume,limitBytes,hostsHomes:form.hostsHomes&&!hasHomes.value}})
  else if(d.kind==='access'){const s=d.target as Share;await storage.storageSetShareAccess({shareId:s.id,ifMatch:'"'+d.version+'"',body:{entries:entries.value.map(({subjectType,subjectId,access})=>({subjectType,subjectId,access}))}})}
  else if(d.kind==='share-quota'){const s=d.target as Share;await storage.storageSetShareQuota({shareId:s.id,ifMatch:'"'+s.version+'"',body:{limitBytes}})}
  else if(d.kind==='measure'){const h=d.target as Home;await storage.storageMeasureHome({userId:h.userId,idempotencyKey:createIdempotencyKey()})}
  else if(d.kind==='share-measure')await storage.storageMeasureShare({shareId:(d.target as Share).id,idempotencyKey:createIdempotencyKey()})
  else if(d.kind==='folder-new')await storage.storageCreateQuotaFolder({shareId:(d.target as Share).id,idempotencyKey:createIdempotencyKey(),body:{path:form.path.split('/').filter(Boolean).join('/'),limitBytes}})
  else if(d.kind==='folder-quota'){const f=d.target as QuotaFolder;await storage.storageSetQuotaFolderQuota({folderId:f.id,ifMatch:'"'+f.version+'"',body:{limitBytes}})}
  else if(d.kind==='folder-remove'){const f=d.target as QuotaFolder;await storage.storageDeleteQuotaFolder({folderId:f.id,ifMatch:'"'+f.version+'"'})}
  else{const h=d.target as Home;await storage.storageSetHomeQuota({userId:h.userId,ifMatch:'"'+h.version+'"',body:{limitBytes}})}
  notice.value=t(d.kind==='measure'||d.kind==='share-measure'?'homes.measureQueued':d.kind==='folder-new'?'folders.created':'saved');dialog.value=null;await load();await loadFolders()
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
   <div v-if="!hasHomes&&!loading&&tab!=='volumes'" class="sm-first-run" role="note"><Info :size="18"/><span><strong>{{t('firstRun.title')}}</strong>{{t('firstRun.body')}}</span></div>

   <div v-if="tab==='shares'" class="sm-list">
    <article v-for="s in shares" :key="s.id" class="sm-card">
     <div class="sm-row"><span class="sm-title"><FolderTree :size="20"/><span><strong>{{s.kind==='homes'?t('shares.homesName'):s.name}}</strong><small>{{s.key}} · {{t('shares.onVolume',{volume:s.volume})}}<template v-if="s.kind==='homes'"> · {{t('shares.hostsHomesBadge')}}</template></small></span></span><span class="sm-actions"><template v-if="s.kind==='share'"><button :disabled="busy" @click="open('access',s)"><ShieldCheck :size="16"/>{{t('shares.access')}}</button><button :disabled="busy" :aria-expanded="openShare===s.id" @click="toggleFolders(s)"><FolderLock :size="16"/>{{t('folders.title')}}</button><button :disabled="busy" @click="open('share-measure',s)"><Ruler :size="16"/>{{t('homes.measure')}}</button></template><button :disabled="busy" @click="open('share-quota',s)"><Gauge :size="16"/>{{t('quota.edit')}}</button></span></div>
     <div class="sm-bar" role="progressbar" :aria-valuenow="percent(s.usage.usedBytes,s.usage.limitBytes)" aria-valuemin="0" aria-valuemax="100"><i :style="{width:percent(s.usage.usedBytes,s.usage.limitBytes)+'%'}"></i></div>
     <small>{{s.usage.measured?t('quota.usedOf',{used:size(s.usage.usedBytes),limit:size(s.usage.limitBytes)}):t('homes.notMeasured')}}<template v-if="s.physicalBytes!=null"> · {{t('quota.physical',{size:size(s.physicalBytes)})}}</template></small>
     <section v-if="openShare===s.id" class="sm-folders" :aria-label="t('folders.title')">
      <p class="sm-muted">{{t('folders.hint')}}</p>
      <div v-for="f in folders" :key="f.id" class="sm-folder">
       <span class="sm-title"><FolderLock :size="16"/><span><strong>{{f.path}}</strong><small>{{f.usage.measured?t('quota.usedOf',{used:size(f.usage.usedBytes),limit:size(f.usage.limitBytes)}):t('folders.notMeasured')}}</small></span></span>
       <div class="sm-bar"><i :style="{width:percent(f.usage.usedBytes,f.usage.limitBytes)+'%'}"></i></div>
       <span class="sm-actions"><button :disabled="busy" @click="open('folder-quota',f)">{{t('quota.edit')}}</button><button :disabled="busy" :aria-label="t('folders.remove')+': '+f.path" @click="open('folder-remove',f)"><Trash2 :size="16"/></button></span>
      </div>
      <p v-if="!folders.length" class="sm-muted">{{t('folders.none')}}</p>
      <button :disabled="busy" @click="open('folder-new',s)"><Plus :size="16"/>{{t('folders.new')}}</button>
     </section>
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
     <label v-if="!hasHomes" class="sm-check"><input v-model="form.hostsHomes" type="checkbox"><span>{{t('shares.hostsHomes')}}<small>{{t('shares.hostsHomesHint')}}</small></span></label>
    </template>
    <template v-else-if="dialog.kind==='access'">
     <p class="sm-muted">{{t('access.hint')}}</p>
     <ul class="sm-access">
      <li v-for="e in entries" :key="e.subjectType+e.subjectId"><span class="sm-title">
       <component :is="e.subjectType==='group'?Users:User" :size="16"/><span><strong>{{e.name||e.subjectId}}</strong><small>{{t('access.'+e.subjectType)}}</small></span></span>
       <select v-model="e.access" :aria-label="t('shares.access')+': '+(e.name||e.subjectId)"><option v-for="a in ['read','write','deny']" :key="a" :value="a">{{t('access.'+a)}}</option></select>
       <button type="button" :aria-label="t('access.remove',{name:e.name||e.subjectId})" :title="t('access.remove',{name:e.name||e.subjectId})" @click="removeEntry(e)"><Trash2 :size="16"/></button>
      </li>
      <li v-if="!entries.length" class="sm-muted">{{t('access.none')}}</li>
     </ul>
     <div class="sm-row"><select v-model="pick" :aria-label="t('access.pick')"><option value="">{{t('access.pick')}}</option><option v-for="s in available" :key="s.type+s.id" :value="s.type+':'+s.id">{{s.name}} ({{t('access.'+s.type)}})</option></select><button type="button" :disabled="!pick||entries.length>=200" @click="addEntry"><Plus :size="16"/>{{t('access.add')}}</button></div>
     <small v-if="entries.length>=200">{{t('access.limit')}}</small>
    </template>
    <template v-else-if="dialog.kind==='folder-new'">
     <p class="sm-muted">{{(dialog.target as Share).name}}</p>
     <label>{{t('folders.path')}}<input v-model="form.path" required maxlength="4096" :placeholder="t('folders.pathExample')"><small>{{t('folders.pathHint')}}</small></label>
    </template>
    <p v-else-if="dialog.kind==='folder-quota'||dialog.kind==='folder-remove'" class="sm-muted">{{(dialog.target as QuotaFolder).path}}<template v-if="dialog.kind==='folder-remove'"><br>{{t('folders.removeHint')}}</template></p>
    <p v-else-if="dialog.kind==='share-measure'" class="sm-muted">{{(dialog.target as Share).name}}<br>{{t('shares.measureHint')}}</p>
    <p v-else class="sm-muted">{{dialog.kind==='share-quota'?(dialog.target as Share).name:(dialog.target as Home).displayName}}</p>
    <label v-if="!['measure','access','share-measure','folder-remove'].includes(dialog.kind)">{{t('quota.limitGB')}}<input v-model="form.quota" inputmode="decimal" :placeholder="t('unlimited')"><small>{{t('quota.limitHint')}}</small></label>
    <label>{{t('confirm.password')}}<input v-model="form.password" type="password" autocomplete="current-password" required maxlength="256"><small>{{t('confirm.reauth')}}</small></label>
   </fieldset>
   <footer class="sm-actions"><button type="button" :disabled="busy" @click="close">{{t('cancel')}}</button><button class="sm-primary" :disabled="busy">{{t(busy?'working':'save')}}</button></footer>
  </form></div>
 </section>
</template>
