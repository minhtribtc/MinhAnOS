<script setup lang="ts">
import {computed,onBeforeUnmount} from 'vue'
import {NotebookPen} from 'lucide-vue-next'
import {useHost} from '@minhanos/app-sdk'
import catalogs from '../locales/catalogs.json'
// Notes (com.minhanos.notes): one private note per account. The host owns storage (the Shell keeps it in the
// account's desktop preferences) and passes state in; this package imports nothing from a Shell.
const props=defineProps<{modelValue:string;displayName?:string;saving?:boolean;saveFailed?:boolean;active?:boolean}>()
const emit=defineEmits<{'update:modelValue':[value:string];retry:[]}>()
const host=useHost()
const release=host.useCatalog('notes',catalogs)
onBeforeUnmount(release)
const t=(key:string,params?:Record<string,string|number>)=>{host.profile.value.locale;return host.i18n.t(key,params)}
const MAX=32000
const count=computed(()=>t('notes.count',{count:host.i18n.formatNumber(props.modelValue.length),max:host.i18n.formatNumber(MAX)}))
defineExpose({canClose:()=>true})
</script>

<template>
  <main class="notes-app">
    <div class="notes-toolbar"><span><NotebookPen :size="16"/> {{displayName?t('notes.owner',{name:displayName}):t('notes.app.name')}}</span><span role="status" :class="{'notes-error':saveFailed}">{{saveFailed?t('notes.saveFailed'):saving?t('notes.saving'):t('notes.autosave')}}</span></div>
    <textarea :value="modelValue" :maxlength="MAX" :aria-label="t('notes.content')" :placeholder="t('notes.placeholder')" @input="emit('update:modelValue',($event.target as HTMLTextAreaElement).value)"></textarea>
    <div class="notes-footer">{{count}} <button v-if="saveFailed" class="notes-retry" @click="emit('retry')">{{t('notes.retry')}}</button></div>
  </main>
</template>
