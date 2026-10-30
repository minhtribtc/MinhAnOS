import {defineComponent,h} from 'vue'
import {defineSettingsProvider,type Request} from '@minhanos/app-sdk'
import manifest from '../../../contracts/manifests/settings.app.json'
import LanguagePanel from './LanguagePanel.vue'

// Platform-owned settings shown by Settings itself (ADR 0007). The Shell supplies its authenticated request.
export function createSystemSettingsProvider(request:Request){
 const language=defineComponent({
  name:'SystemSettingsLanguage',
  props:{active:{type:Boolean,default:true}},
  setup:props=>()=>h(LanguagePanel,{request,active:props.active})
 })
 return defineSettingsProvider({manifest,panels:{language}})
}
