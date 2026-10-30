import common from '../locales/common.json' with {type:'json'}
import {inject,ref,watch,readonly} from 'vue'
import {createI18n,resolveLocale} from '@minhanos/i18n'
import {resolveTheme} from '@minhanos/ui'
export const sdkVersion=1
const hostKey=Symbol('minhanos.host.v1')
export class ApiError extends Error {
 constructor(status,body={}){super(typeof body.error==='string'?body.error:'Request failed');this.name='ApiError';this.status=status;this.code=typeof body.code==='string'?body.code:'REQUEST_FAILED';this.requestId=typeof body.requestId==='string'?body.requestId:undefined}
}
export function createApiClient(base='/api'){
 let csrf=''
 return {setCSRF(value){csrf=value},async request(path,options={}){
  const headers=new Headers(options.headers);if(!headers.has('Content-Type'))headers.set('Content-Type','application/json');headers.set('X-CSRF-Token',csrf)
  const response=await fetch(base+path,{...options,credentials:'same-origin',headers})
  const body=await response.json().catch(()=>({code:'INVALID_RESPONSE'}))
  if(!response.ok)throw new ApiError(response.status,body)
  return body
 }}
}
export function createHost(){
 let guest;try{guest=localStorage.getItem('minhanos.locale')||undefined}catch{}
 const profile=ref({locale:resolveLocale(undefined,guest,globalThis.navigator?.languages||[]),theme:'light',timeZone:'Asia/Bangkok'})
 const i18n=createI18n(profile.value.locale,profile.value.timeZone),systemDark=ref(false)
 const media=globalThis.matchMedia?.('(prefers-color-scheme: dark)')
 if(media)systemDark.value=media.matches
 const onMedia=()=>{systemDark.value=!!media?.matches};media?.addEventListener('change',onMedia)
 i18n.registerCatalog('common',common)
 const stop=watch([profile,systemDark],()=>{i18n.setLocale(profile.value.locale);i18n.setTimeZone(profile.value.timeZone);if(globalThis.document){document.documentElement.dataset.mnTheme=resolveTheme(profile.value.theme,systemDark.value);document.documentElement.lang=profile.value.locale}},{immediate:true,deep:true,flush:'sync'})
 let updating=false, accountProfile
 function applyProfile(value){
  if(!value || typeof value.locale!=='string' || typeof value.theme!=='string' || typeof value.timeZone!=='string')throw new ApiError(502,{code:'INVALID_RESPONSE'})
  accountProfile={locale:value.locale,theme:value.theme,timeZone:value.timeZone}
  let timeZone=value.timeZone
  try{new Intl.DateTimeFormat('en-US',{timeZone})}catch{timeZone='Asia/Bangkok'}
  profile.value={locale:resolveLocale(value.locale,guest,globalThis.navigator?.languages||[]),theme:['light','dark','system'].includes(value.theme)?value.theme:'light',timeZone}
 }
 return {errorMessage(error){const key='common.errors.'+(error instanceof ApiError&&('common.errors.'+error.code) in common['vi-VN']?error.code:'REQUEST_FAILED');return i18n.t(key)},version:sdkVersion,profile:readonly(profile),i18n,
  install(app){app.provide(hostKey,this);app.onUnmount(()=>this.dispose())},
  applyProfile,
  async loadProfile(request){applyProfile(await request('/profile'))},
  async updateProfile(request,patch){
   if(updating)throw new Error('PROFILE_UPDATE_PENDING')
   updating=true
   try{const saved=await request('/profile',{method:'PUT',body:JSON.stringify({...(accountProfile??profile.value),...patch})});applyProfile(saved);try{localStorage.setItem('minhanos.locale',saved.locale)}catch{}}finally{updating=false}
  },
  dispose(){stop();media?.removeEventListener('change',onMedia)}
 }
}
export function useHost(){const host=inject(hostKey);if(!host)throw new Error('MinhAnOS host provider required');return host}
