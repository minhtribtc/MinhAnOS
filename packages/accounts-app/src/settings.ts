import {defineComponent,h,ref} from 'vue'
import {defineSettingsProvider} from '@minhanos/app-sdk'
import manifest from '../../../contracts/manifests/accounts.app.json'
import AccountsApp from './index'

type Section='profile'|'security'|'sessions'|'users'

// Each Settings entry reuses the Account Center screen in panel mode (no own sidebar/logout).
function panel(section:Section){
 return defineComponent({
  name:'AccountsSettings_'+section,
  props:{active:{type:Boolean,default:true}},
  setup(props,{expose}){
   const inner=ref<InstanceType<typeof AccountsApp>|null>(null)
   expose({canClose:()=>inner.value?.canClose()??true})
   return ()=>h(AccountsApp,{ref:inner,embedded:true,panel:true,section,active:props.active})
  }
 })
}

export const settingsProvider=defineSettingsProvider({
 manifest,
 panels:{profile:panel('profile'),security:panel('security'),sessions:panel('sessions'),users:panel('users')}
})
