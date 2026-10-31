<script setup lang="ts">
// The page of a public link /s/<token> (ADR 0011), without an account: unlock with the password, then browse and
// download (view), also upload and create folders (edit), or only upload into the shared folder (drop).
import {computed,onBeforeUnmount,onMounted,ref} from 'vue'
import {Folder,FolderOpen,FolderPlus,FileText,Download,Upload,ChevronRight,LoaderCircle,Lock,Link2,Check,X} from 'lucide-vue-next'
import {useHost} from '@minhanos/app-sdk'
import {createIdempotencyKey} from '@minhanos/api-client'
import catalogs from '../locales/catalogs.json'
import {bytes,type Entry} from './client'
const props=withDefaults(defineProps<{apiBase?:string}>(),{apiBase:'/api'})
const host=useHost()
const releaseCatalog=host.useCatalog('files',catalogs)
onBeforeUnmount(releaseCatalog)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t(key,params)}
type Info={name:string;dir:boolean;permission:'view'|'edit'|'drop';owner:string;needsPassword:boolean;expires:string|null;maxUploadBytes:number}
type Sent={id:number;name:string;progress:number;status:'uploading'|'done'|'error';error:string}
const token=location.pathname.split('/')[2]??''
const base=computed(()=>props.apiBase+'/public/'+encodeURIComponent(token))
const info=ref<Info|null>(null),state=ref<'loading'|'ready'|'missing'|'expired'>('loading'),error=ref(''),password=ref(''),busy=ref(false)
const path=ref(''),entries=ref<Entry[]>([]),loading=ref(false),sent=ref<Sent[]>([]),folderName=ref(''),creating=ref(false)
const input=ref<HTMLInputElement|null>(null)
const crumbs=computed(()=>path.value.split('/').filter(Boolean))
const canList=computed(()=>info.value?.permission!=='drop')
const canUpload=computed(()=>!!info.value?.dir&&info.value.permission!=='view')
const sorted=computed(()=>[...entries.value].sort((a,b)=>(a.kind==='folder')!==(b.kind==='folder')?(a.kind==='folder'?-1:1):a.name.localeCompare(b.name,host.profile.value.locale,{numeric:true,sensitivity:'base'})))
const longDate=(value:string)=>host.i18n.formatDate(new Date(value),{dateStyle:'medium'})
function message(code:string){const key='files.errors.'+code;return key in catalogs['vi-VN']?t(key):t('files.public.failed')}
async function call<T>(op:string,options:RequestInit={}):Promise<T>{const headers=new Headers(options.headers);if(options.method&&options.method!=='GET'){headers.set('Idempotency-Key',createIdempotencyKey());if(!headers.has('Content-Type'))headers.set('Content-Type','application/json')}const r=await fetch(base.value+op,{...options,headers,credentials:'same-origin'});const body=await r.json().catch(()=>({}));if(!r.ok)throw Object.assign(new Error(body.code||'INVALID_RESPONSE'),{status:r.status,code:body.code||'INVALID_RESPONSE'});return body as T}
const files=(op:string,p:string,extra:Record<string,string>={})=>`/files/${op}?${new URLSearchParams({path:p,...extra})}`
async function load(){state.value='loading';error.value='';try{info.value=await call<Info>('');state.value='ready';if(!info.value.needsPassword&&canList.value)await list('')}catch(e){const code=(e as {code?:string}).code;state.value=code==='FILES_LINK_EXPIRED'?'expired':'missing'}}
async function unlock(){busy.value=true;error.value='';try{await call('/unlock',{method:'POST',body:JSON.stringify({password:password.value})});password.value='';await load()}catch(e){error.value=message((e as {code:string}).code)}finally{busy.value=false}}
async function list(p:string){loading.value=true;error.value='';try{const all:Entry[]=[];let cursor='';do{const page=await call<{entries:Entry[];nextCursor:string|null}>(files('list',p,cursor?{cursor}:{}));all.push(...page.entries);cursor=page.nextCursor??''}while(cursor);entries.value=all;path.value=p}catch(e){error.value=message((e as {code:string}).code)}finally{loading.value=false}}
const downloadURL=(e:Entry)=>base.value+(e.kind==='folder'?files('archive','',{item:e.path}):files('download',e.path))
const allURL=computed(()=>base.value+`/files/archive?${new URLSearchParams([['path',''],...entries.value.filter(e=>e.kind==='file'||e.kind==='folder').map(e=>['item',e.path])])}`)
function upload(list:FileList|null){if(!list||!info.value)return;const target=path.value;for(const file of Array.from(list)){const item:Sent={id:Date.now()+Math.random(),name:file.name,progress:0,status:'uploading',error:''};sent.value.unshift(item);const row=sent.value[0];if(file.size>info.value.maxUploadBytes){row.status='error';row.error=t('files.notice.tooLarge',{name:file.name});continue}
 const xhr=new XMLHttpRequest();xhr.open('POST',base.value+files('upload',target,{name:file.name}));xhr.withCredentials=true;xhr.setRequestHeader('Idempotency-Key',createIdempotencyKey());xhr.setRequestHeader('Content-Type','application/octet-stream');xhr.upload.onprogress=e=>{if(e.lengthComputable)row.progress=Math.round(e.loaded/e.total*100)};xhr.onerror=()=>{row.status='error';row.error=t('files.public.failed')};xhr.onload=()=>{if(xhr.status>=200&&xhr.status<300){row.status='done';row.progress=100;if(canList.value&&path.value===target)void list(target)}else{row.status='error';let code='';try{code=JSON.parse(xhr.responseText).code}catch{}row.error=message(code)}};xhr.send(file)}
 if(input.value)input.value.value=''}
async function mkdir(){if(!folderName.value.trim())return;busy.value=true;error.value='';try{await call(files('mkdir',path.value),{method:'POST',body:JSON.stringify({name:folderName.value.trim()})});folderName.value='';creating.value=false;await list(path.value)}catch(e){error.value=message((e as {code:string}).code)}finally{busy.value=false}}
function drop(e:DragEvent){e.preventDefault();if(canUpload.value)upload(e.dataTransfer?.files??null)}
onMounted(load)
</script>

<template>
 <div class="files-app public-share" @dragover.prevent @drop="drop">
  <header class="drive-top"><span class="drive-brand"><span><FolderOpen :size="21"/></span><strong>{{t('files.app.name')}}</strong></span></header>
  <main v-if="state==='loading'" class="login-wrap"><LoaderCircle class="spinning"/> {{t('files.loading')}}</main>
  <main v-else-if="state!=='ready'||!info" class="login-wrap"><section class="drive-login" role="alert"><span class="login-folder"><Link2 :size="36"/></span><h1>{{state==='expired'?t('files.public.expired'):t('files.public.missing')}}</h1><p>{{state==='expired'?t('files.public.expiredHint'):t('files.public.missingHint')}}</p></section></main>
  <main v-else-if="info.needsPassword" class="login-wrap"><section class="drive-login"><span class="login-folder"><Lock :size="36"/></span><div class="eyebrow">{{t('files.public.from',{owner:info.owner})}}</div><h1>{{info.name}}</h1><p>{{t('files.public.passwordHint')}}</p><form @submit.prevent="unlock"><label>{{t('files.public.password')}}<input v-model="password" type="password" autocomplete="off" required maxlength="256" autofocus></label><p v-if="error" role="alert" class="error">{{error}}</p><button class="primary" :disabled="busy">{{t('files.public.unlock')}}<ChevronRight :size="17"/></button></form></section></main>
  <main v-else class="public-main">
   <div class="drive-heading"><div><span class="eyebrow">{{t('files.public.from',{owner:info.owner})}}</span><h1>{{info.name}}</h1><p>{{t('files.public.mode.'+info.permission)}}<template v-if="info.expires"> · {{t('files.public.expires',{date:longDate(info.expires)})}}</template></p></div>
    <div v-if="canUpload||(canList&&entries.length)" class="heading-actions"><a v-if="canList&&entries.length" class="button" :href="allURL" download><Download :size="17"/>{{info.dir?t('files.public.downloadAll'):t('files.actions.download')}}</a><button v-if="canUpload&&info.permission==='edit'" @click="creating=!creating"><FolderPlus :size="17"/>{{t('files.actions.mkdir')}}</button><button v-if="canUpload" class="primary" @click="input?.click()"><Upload :size="17"/>{{t('files.actions.upload')}}</button></div>
    <input ref="input" type="file" multiple hidden @change="upload(($event.target as HTMLInputElement).files)"></div>
   <form v-if="creating" class="public-mkdir" @submit.prevent="mkdir"><input v-model="folderName" :aria-label="t('files.dialog.name')" :placeholder="t('files.dialog.name')" required maxlength="255" autofocus><button class="primary" :disabled="busy">{{t('files.dialog.confirm')}}</button><button type="button" @click="creating=false">{{t('files.dialog.cancel')}}</button></form>
   <div v-if="error" class="error-banner" role="alert"><span>{{error}}</span><button :aria-label="t('files.error.dismiss')" @click="error=''"><X :size="16"/></button></div>
   <section v-if="info.permission==='drop'" class="public-drop" @click="input?.click()"><Upload :size="40"/><h2>{{t('files.public.dropTitle')}}</h2><p>{{t('files.public.dropHint')}}</p></section>
   <template v-else>
    <div v-if="info.dir" class="breadcrumbs"><button @click="list('')"><Folder :size="15"/>{{info.name}}</button><template v-for="(part,index) in crumbs" :key="index"><ChevronRight :size="12"/><button @click="list(crumbs.slice(0,index+1).join('/'))">{{part}}</button></template></div>
    <div v-if="loading" class="empty-files"><LoaderCircle class="spinning" :size="28"/></div>
    <div v-else-if="!sorted.length" class="empty-files"><FolderOpen :size="44"/><h2>{{t('files.browser.empty')}}</h2></div>
    <ul v-else class="public-list"><li v-for="e in sorted" :key="e.path"><button v-if="e.kind==='folder'" class="public-name" @click="list(e.path)"><Folder :size="22"/><span>{{e.name}}</span></button><span v-else class="public-name"><FileText :size="22"/><span>{{e.name}}</span></span><small>{{e.kind==='folder'?t('files.kind.folder'):bytes(e.size)}}</small><a :href="downloadURL(e)" :aria-label="t('files.public.downloadItem',{name:e.name})" download><Download :size="17"/></a></li></ul>
   </template>
   <section v-if="sent.length" class="public-sent" :aria-label="t('files.transfers.title')"><div v-for="s in sent" :key="s.id" class="transfer-row"><FileText :size="20"/><div><strong>{{s.name}}</strong><small>{{s.status==='done'?t('files.transfers.done'):s.status==='error'?s.error:t('files.transfers.sending',{progress:s.progress})}}</small><div class="progress"><i :style="{width:s.progress+'%'}"></i></div></div><Check v-if="s.status==='done'" :size="17"/></div></section>
   <p class="muted public-footer">{{t('files.public.footer')}}</p>
  </main>
 </div>
</template>
