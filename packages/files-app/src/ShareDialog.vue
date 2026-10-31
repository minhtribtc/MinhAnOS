<script setup lang="ts">
// Share an item of the personal home (ADR 0011): with users of a common organization (view or edit) and by public
// links (view; for folders also edit = upload and new folder, or drop = upload only), with optional password and expiry.
import {computed,onMounted,ref} from 'vue'
import {X,Users,Link2,Copy,Trash2,LoaderCircle,Check} from 'lucide-vue-next'
type Grant={id:string;path:string;name:string;dir:boolean;permission:'view'|'edit';granteeId:string;grantee:string}
type Link={id:string;token:string;path:string;dir:boolean;permission:'view'|'edit'|'drop';hasPassword:boolean;expires:string|null;uses:number;lastUsed:string|null}
type Recipient={id:string;username:string;displayName:string}
const props=defineProps<{path:string;name:string;dir:boolean;publicURL:(token:string)=>string;request:<T>(path:string,options?:RequestInit)=>Promise<T>;t:(key:string,params?:Record<string,string|number>)=>string;message:(e:unknown)=>string}>()
const emit=defineEmits<{close:[];changed:[]}>()
const grants=ref<Grant[]>([]),links=ref<Link[]>([]),loading=ref(true),busy=ref(false),error=ref(''),copied=ref('')
const query=ref(''),recipients=ref<Recipient[]>([]),chosen=ref<Recipient|null>(null),grantPermission=ref<'view'|'edit'>('view')
const linkPermission=ref<'view'|'edit'|'drop'>('view'),linkPassword=ref(''),linkExpires=ref('')
const today=new Date().toISOString().slice(0,10)
const t=props.t
const post=<T=unknown>(path:string,body:object)=>props.request<T>(path,{method:'POST',body:JSON.stringify(body)})
async function load(){loading.value=true;try{const r=await props.request<{grants:Grant[];links:Link[]}>('/sharing?path='+encodeURIComponent(props.path));grants.value=r.grants;links.value=r.links}catch(e){error.value=props.message(e)}finally{loading.value=false}}
let searchTimer=0
function search(){chosen.value=null;clearTimeout(searchTimer);searchTimer=window.setTimeout(async()=>{try{recipients.value=(await props.request<{items:Recipient[]}>('/sharing/recipients?q='+encodeURIComponent(query.value.trim()))).items.filter(r=>!grants.value.some(g=>g.granteeId===r.id))}catch(e){error.value=props.message(e)}},250)}
function pick(r:Recipient){chosen.value=r;query.value=r.displayName;recipients.value=[]}
async function run(action:()=>Promise<unknown>){busy.value=true;error.value='';try{await action();await load();emit('changed')}catch(e){error.value=props.message(e)}finally{busy.value=false}}
const addGrant=()=>chosen.value&&run(async()=>{await post('/sharing/grants',{path:props.path,recipient:chosen.value!.id,permission:grantPermission.value});chosen.value=null;query.value=''})
const setGrant=(g:Grant,permission:string)=>run(()=>post('/sharing/grants',{path:props.path,recipient:g.granteeId,permission}))
const removeGrant=(g:Grant)=>run(()=>post('/sharing/grants/remove',{id:g.id}))
// The expiry date ends at the close of that day (local time).
const expiry=(day:string)=>day?new Date(day+'T23:59:59').toISOString():''
const addLink=()=>run(async()=>{await post('/sharing/links',{path:props.path,permission:linkPermission.value,password:linkPassword.value,expires:expiry(linkExpires.value)});linkPassword.value='';linkExpires.value=''})
const updateLink=(l:Link,change:{permission?:string;expires?:string;clearPassword?:boolean})=>run(()=>post('/sharing/links/update',{id:l.id,permission:change.permission??l.permission,expires:change.expires!==undefined?expiry(change.expires):l.expires??'',clearPassword:!!change.clearPassword}))
const removeLink=(l:Link)=>run(()=>post('/sharing/links/remove',{id:l.id}))
async function copy(l:Link){try{await navigator.clipboard.writeText(props.publicURL(l.token));copied.value=l.id;setTimeout(()=>copied.value='',2000)}catch{error.value=t('files.share.copyFailed')}}
const linkPermissions=computed(()=>props.dir?['view','edit','drop'] as const:['view'] as const)
onMounted(load)
</script>

<template>
 <div class="modal-scrim"><section class="share-dialog" role="dialog" aria-modal="true" aria-labelledby="files-share-title">
  <header><h2 id="files-share-title">{{t('files.share.title',{name})}}</h2><button type="button" :aria-label="t('files.dialog.cancel')" @click="emit('close')"><X :size="18"/></button></header>
  <p v-if="error" class="error" role="alert">{{error}}</p>
  <div v-if="loading" class="share-loading"><LoaderCircle class="spinning" :size="22"/></div>
  <template v-else>
   <h3><Users :size="16"/>{{t('files.share.people')}}</h3>
   <form class="share-add" @submit.prevent="addGrant">
    <div class="share-search"><input v-model="query" :aria-label="t('files.share.findPerson')" :placeholder="t('files.share.findPerson')" autocomplete="off" @input="search" @focus="search">
     <ul v-if="recipients.length&&!chosen" class="share-suggestions" role="listbox"><li v-for="r in recipients" :key="r.id"><button type="button" role="option" :aria-selected="false" @click="pick(r)">{{r.displayName}} <small>@{{r.username}}</small></button></li></ul></div>
    <select v-model="grantPermission" :aria-label="t('files.share.permission')"><option value="view">{{t('files.share.view')}}</option><option value="edit">{{t('files.share.edit')}}</option></select>
    <button class="primary" :disabled="!chosen||busy">{{t('files.share.add')}}</button>
   </form>
   <p v-if="!grants.length" class="muted">{{t('files.share.noPeople')}}</p>
   <ul class="share-list"><li v-for="g in grants" :key="g.id"><span>{{g.grantee}}</span><select :value="g.permission" :aria-label="t('files.share.permissionFor',{name:g.grantee})" :disabled="busy" @change="setGrant(g,($event.target as HTMLSelectElement).value)"><option value="view">{{t('files.share.view')}}</option><option value="edit">{{t('files.share.edit')}}</option></select><button type="button" :aria-label="t('files.share.removePerson',{name:g.grantee})" :disabled="busy" @click="removeGrant(g)"><Trash2 :size="15"/></button></li></ul>
   <h3><Link2 :size="16"/>{{t('files.share.links')}}</h3>
   <ul class="share-list links"><li v-for="l in links" :key="l.id">
    <div class="share-link-url"><code>{{publicURL(l.token)}}</code><button type="button" :aria-label="t('files.share.copy')" @click="copy(l)"><component :is="copied===l.id?Check:Copy" :size="15"/></button></div>
    <div class="share-link-meta">
     <select :value="l.permission" :aria-label="t('files.share.permission')" :disabled="busy" @change="updateLink(l,{permission:($event.target as HTMLSelectElement).value})"><option v-for="p in linkPermissions" :key="p" :value="p">{{t('files.share.link.'+p)}}</option></select>
     <label>{{t('files.share.expires')}} <input type="date" :min="today" :value="l.expires?l.expires.slice(0,10):''" :disabled="busy" @change="updateLink(l,{expires:($event.target as HTMLInputElement).value})"></label>
     <button v-if="l.hasPassword" type="button" :disabled="busy" @click="updateLink(l,{clearPassword:true})">{{t('files.share.removePassword')}}</button>
     <small>{{l.hasPassword?t('files.share.protected'):t('files.share.open')}} · {{t('files.share.uses',{count:l.uses})}}</small>
     <button type="button" class="danger-text" :aria-label="t('files.share.removeLink')" :disabled="busy" @click="removeLink(l)"><Trash2 :size="15"/></button>
    </div></li></ul>
   <form class="share-new-link" @submit.prevent="addLink">
    <select v-model="linkPermission" :aria-label="t('files.share.permission')"><option v-for="p in linkPermissions" :key="p" :value="p">{{t('files.share.link.'+p)}}</option></select>
    <input v-model="linkPassword" type="password" autocomplete="new-password" minlength="6" maxlength="256" :aria-label="t('files.share.password')" :placeholder="t('files.share.passwordOptional')">
    <label>{{t('files.share.expires')}} <input v-model="linkExpires" type="date" :min="today"></label>
    <button class="primary" :disabled="busy">{{t('files.share.createLink')}}</button>
   </form>
   <p class="muted">{{t('files.share.linkHint')}}</p>
  </template>
 </section></div>
</template>
