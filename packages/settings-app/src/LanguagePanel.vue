<script setup lang="ts">
import {computed,ref} from 'vue'
import {useHost,type Request} from '@minhanos/app-sdk'

// Platform-owned setting (system.locale/timeZone via the profile API). Every app follows it through app-sdk.
const props=withDefaults(defineProps<{request:Request;active?:boolean}>(),{active:true})
const host=useHost()
const t=(key:string,params?:Record<string,string>)=>{host.profile.value.locale;return host.i18n.t(key,params)}
const busy=ref(false),error=ref('')
const zones=(()=>{try{return Intl.supportedValuesOf('timeZone')}catch{return ['Asia/Bangkok','Asia/Ho_Chi_Minh','UTC']}})()
const sample=computed(()=>{host.profile.value.locale;host.profile.value.timeZone;return host.i18n.formatDate(Date.now(),{dateStyle:'full',timeStyle:'short'})})
async function save(field:'locale'|'timeZone',event:Event){
 if(busy.value)return
 busy.value=true;error.value=''
 try{await host.updateProfile(props.request,{[field]:(event.target as HTMLSelectElement).value})}
 catch(e){error.value=host.errorMessage(e)}
 finally{busy.value=false}
}
</script>

<template>
 <div class="st-page">
  <h2>{{t('settings.language.title')}}</h2>
  <p class="st-muted">{{t('settings.language.hint')}}</p>
  <div class="st-rows">
   <label class="st-row"><span>{{t('settings.language.locale')}}</span><select :value="host.profile.value.locale" :disabled="busy" @change="save('locale',$event)"><option value="vi-VN">Tiếng Việt</option><option value="en-US">English</option></select></label>
   <label class="st-row"><span>{{t('settings.language.timeZone')}}</span><select :value="host.profile.value.timeZone" :disabled="busy" @change="save('timeZone',$event)"><option v-for="zone in zones" :key="zone" :value="zone">{{zone}}</option></select></label>
   <div class="st-row"><span>{{t('settings.language.preview')}}</span><strong>{{sample}}</strong></div>
  </div>
  <p v-if="error" class="st-error" role="alert">{{error}}</p>
 </div>
</template>
