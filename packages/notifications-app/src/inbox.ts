import {ref} from 'vue'
import {createApiClient,createIdempotencyKey,ApiError} from '@minhanos/app-sdk'
import {createNotificationsClient} from '@minhanos/api-client/notifications'
import type {NotificationItem,NotificationMarkRead} from '@minhanos/shared-types'
import catalogs from '../locales/catalogs.json'

export type Notice=NotificationItem
export type Inbox=ReturnType<typeof createInbox>

// One inbox state for the Notification Center app and the Shell's bell panel. Polling only (v1).
export function createInbox(apiBase='/api'){
 const api=createApiClient(apiBase),client=createNotificationsClient(apiBase+'/v1/notifications')
 const items=ref<Notice[]>([]),unread=ref(0),next=ref<string|null>(null),loading=ref(false),error=ref<unknown>(null)
 let csrf:Promise<void>|null=null
 const ready=()=>csrf??=api.request<{csrfToken:string}>('/session').then(s=>client.setCSRF(s.csrfToken)).catch(e=>{csrf=null;throw e})
 async function load(options:{unreadOnly?:boolean;limit?:number;more?:boolean}={}){
  if(loading.value)return
  loading.value=true;error.value=null
  try{
   const page=await client.notificationsListInbox({limit:options.limit??20,unread:options.unreadOnly?'true':undefined,cursor:options.more?next.value??undefined:undefined})
   items.value=options.more?[...items.value,...page.items]:page.items
   next.value=page.nextCursor;unread.value=page.unreadCount
  }catch(e){error.value=e}finally{loading.value=false}
 }
 async function settle(body:NotificationMarkRead){
  try{
   await ready()
   const result=await client.notificationsMarkRead({idempotencyKey:createIdempotencyKey(),body})
   const now=new Date().toISOString()
   items.value=items.value.map(n=>!n.readAt&&(body.ids?body.ids.includes(n.id):Date.parse(n.createdAt)<=Date.parse(body.upTo??''))?{...n,readAt:now}:n)
   unread.value=result.unreadCount
  }catch(e){error.value=e}
 }
 const markRead=(ids:string[])=>ids.length?settle({ids:ids as [string,...string[]]}):Promise.resolve()
 // "Mark all" covers what the user has seen: notices that arrive later stay unread.
 const markAll=()=>items.value.length?settle({upTo:items.value[0].createdAt}):Promise.resolve()
 return {items,unread,next,loading,error,load,markRead,markAll}
}

type T=(key:string,params?:Record<string,string|number>)=>string
const known=(key:string)=>key in catalogs['vi-VN']

/** Title and body from i18n keys (source + kind); notices never carry server text. */
export function noticeText(t:T,n:Notice){
 const base='notifications.kind.'+n.kind,params={...n.params,kind:n.kind}
 return known(base+'.title')?{title:t(base+'.title',params),body:t(base+'.body',params)}:{title:t('notifications.kind.unknown.title',params),body:t('notifications.kind.unknown.body',params)}
}

export function sourceName(t:T,source:string){
 const key='notifications.sources.'+source.replace('com.minhanos.','')
 return known(key)?t(key):source
}

export function errorText(t:T,host:{errorMessage(e:unknown):string},e:unknown){
 if(e instanceof ApiError){const key='notifications.errors.'+e.code;return known(key)?t(key):host.errorMessage(e)}
 return t('notifications.errors.network')
}
