<script setup lang="ts">
import {computed,onBeforeUnmount,ref,watch,type Component,type ComponentPublicInstance} from 'vue'
import {Search,Settings} from 'lucide-vue-next'
import {useHost,type SettingsProvider} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
import type {SettingsTarget} from './types'

// Central Settings shell: navigation comes only from manifest settingsIntegration entries
// and panels are components exported by each app package (see AGENTS.md Central Settings standard).
interface Entry {key:string;appId:string;appName:string;id:string;label:string;component?:Component}

const props=withDefaults(defineProps<{providers:SettingsProvider[];permissions:readonly string[]|null;active?:boolean;target?:SettingsTarget|null}>(),{active:true,target:null})
const host=useHost(),release=host.useCatalog('settings',catalogs)
onBeforeUnmount(release)
const t=(key:string)=>{host.profile.value.locale;return host.i18n.t(key)}

const query=ref(''),selected=ref(''),visited=ref<string[]>([])
const instances=new Map<string,ComponentPublicInstance&{canClose?:()=>boolean}>()

// Permission filtering is cosmetic; every panel's backend still enforces access.
const entries=computed<Entry[]>(()=>props.providers.flatMap(provider=>provider.manifest.settingsIntegration
 .filter(entry=>!entry.requiredPermission||props.permissions?.includes(entry.requiredPermission))
 .map(entry=>({key:provider.manifest.id+'/'+entry.id,appId:provider.manifest.id,appName:provider.manifest.nameKey?t(provider.manifest.nameKey):provider.manifest.id,id:entry.id,label:t(entry.labelKey),component:provider.panels[entry.id]}))))
const filtered=computed(()=>{
 const needle=query.value.trim().toLocaleLowerCase(host.profile.value.locale)
 if(!needle)return entries.value
 return entries.value.filter(e=>(e.label+' '+e.appName).toLocaleLowerCase(host.profile.value.locale).includes(needle))
})
const groups=computed(()=>{
 const result:{appId:string;appName:string;items:Entry[]}[]=[]
 for(const entry of filtered.value){
  let group=result.find(g=>g.appId===entry.appId)
  if(!group){group={appId:entry.appId,appName:entry.appName,items:[]};result.push(group)}
  group.items.push(entry)
 }
 return result
})
const current=computed(()=>entries.value.find(e=>e.key===selected.value))
const mounted=computed(()=>entries.value.filter(e=>visited.value.includes(e.key)))

function select(key:string){selected.value=key;if(!visited.value.includes(key))visited.value=[...visited.value,key]}
function setInstance(key:string,el:unknown){if(el)instances.set(key,el as ComponentPublicInstance&{canClose?:()=>boolean});else instances.delete(key)}

watch(entries,list=>{
 if(!list.some(e=>e.key===selected.value)&&list.length)select(list[0].key)
 visited.value=visited.value.filter(key=>list.some(e=>e.key===key))
},{immediate:true})
watch(()=>props.target?.seq,()=>{
 const target=props.target;if(!target)return
 const match=entries.value.find(e=>e.appId===target.appId&&(!target.panelId||e.id===target.panelId))
 if(match){query.value='';select(match.key)}
},{immediate:true})

// Panels stay mounted after the first visit so drafts survive switching panels; closing asks every one.
function canClose(){return [...instances.values()].every(panel=>panel.canClose?.()??true)}
defineExpose({canClose})
</script>

<template>
 <section class="mn-settings" :aria-label="t('settings.title')">
  <aside class="st-sidebar">
   <h1><Settings :size="20"/>{{t('settings.title')}}</h1>
   <label class="st-search"><Search :size="16"/><input v-model="query" type="search" :placeholder="t('settings.search')" :aria-label="t('settings.search')" maxlength="100"></label>
   <nav :aria-label="t('settings.title')">
    <div v-for="group in groups" :key="group.appId" class="st-group">
     <span class="st-group-name">{{group.appName}}</span>
     <button v-for="entry in group.items" :key="entry.key" :class="{selected:entry.key===selected}" :aria-current="entry.key===selected?'page':undefined" @click="select(entry.key)">{{entry.label}}</button>
    </div>
   </nav>
   <p v-if="!groups.length" class="st-muted">{{t(entries.length?'settings.noResults':'settings.empty')}}</p>
  </aside>
  <main class="st-main">
   <p v-if="current" class="st-crumb">{{current.appName}} › {{current.label}}</p>
   <div v-for="entry in mounted" v-show="entry.key===selected" :key="entry.key" class="st-panel">
    <component :is="entry.component" v-if="entry.component" :ref="(el:unknown)=>setInstance(entry.key,el)" :active="active&&entry.key===selected"/>
    <p v-else class="st-muted">{{t('settings.missingPanel')}}</p>
   </div>
  </main>
 </section>
</template>
