import {createApiClient,ApiError} from '@minhanos/api-client'
import type {DesktopAppList,DesktopPreferences,DesktopSession,DesktopSystemStatus} from '@minhanos/shared-types'
// DTOs are generated from contracts/desktop (OpenAPI); see packages/shared-types.
export type User = DesktopSession['user']
// Launcher entry from GET /api/apps; names and descriptions come from the app's manifest catalog (registry.ts).
export type AppDefinition = DesktopAppList[number]
export type Preferences = DesktopPreferences
export type DesktopWindow = Preferences['windows'][number]
export type SystemInfo = DesktopSystemStatus

const client=createApiClient('/api')
let authVersion=0
let authFailure:(()=>void)|undefined
export function onAuthFailure(listener:()=>void){authFailure=listener;return ()=>{if(authFailure===listener)authFailure=undefined}}
let csrf='',preferencesTag=''
export function setCSRF(value:string){authVersion++;csrf=value;client.setCSRF(value)}
export async function api<T=unknown>(path:string,options?:RequestInit):Promise<T>{
 const version=authVersion
 try{return await client.request<T>(path,options)}catch(error){
  if(version===authVersion&&error instanceof ApiError&&error.code==='AUTH_REQUIRED')authFailure?.()
  throw error
 }
}
// Preferences are versioned (API contract): GET returns an ETag, PUT sends it as If-Match and gets the next one.
// A 412 means another tab saved first; the tag is refreshed and this tab's newer layout is saved once more.
async function preferencesCall<T>(method:'GET'|'PUT',body?:string):Promise<T>{
 const version=authVersion
 const headers=new Headers({'Content-Type':'application/json','X-CSRF-Token':csrf})
 if(method==='PUT')headers.set('If-Match',preferencesTag)
 const response=await fetch('/api/preferences',{method,body,headers,credentials:'same-origin'})
 const data=await response.json().catch(()=>({code:'INVALID_RESPONSE'}))
 if(!response.ok){
  const error=new ApiError(response.status,data)
  if(version===authVersion&&error.code==='AUTH_REQUIRED')authFailure?.()
  throw error
 }
 preferencesTag=response.headers?.get('ETag')??preferencesTag
 return data
}
export const loadPreferences=()=>preferencesCall<Preferences>('GET')
export async function savePreferences(body:string){
 try{await preferencesCall('PUT',body)}catch(error){
  if(!(error instanceof ApiError)||error.status!==412)throw error
  await loadPreferences()
  await preferencesCall('PUT',body)
 }
}
export function bytes(value: number) {
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(0)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(1)} GB`
}
