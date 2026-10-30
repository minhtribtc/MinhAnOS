import type {App,Component,DeepReadonly,Ref} from 'vue'
import type {Catalogs,I18n,Locale} from '@minhanos/i18n'
import type {AccountProfile} from '@minhanos/shared-types'
export type Profile=AccountProfile
export type Request=<T=any>(path:string,options?:RequestInit)=>Promise<T>
export {ApiError,createApiClient} from '@minhanos/api-client'
/** Host-wide notifications shared by every app rendered in the same Shell. */
export interface HostEvents {'session-ended':void;'profile-changed':{displayName:string}}
export interface SettingsEntry {id:string;labelKey:string;path:string;requiredPermission?:string}
export interface SettingsManifest {id:string;nameKey?:string;settingsIntegration:SettingsEntry[]}
/** Panels receive `active`; they may expose `canClose()` to protect unsaved drafts. */
export interface SettingsPanelProps {active:boolean}
export interface SettingsProvider {manifest:SettingsManifest;panels:Record<string,Component>}
export interface Host {errorMessage(error:unknown):string;version:number;profile:DeepReadonly<Ref<Profile>>;i18n:I18n;install(app:App):void;applyProfile(value:Profile):void;loadProfile(request:Request):Promise<void>;updateProfile(request:Request,patch:Partial<Profile>):Promise<void>;dispose():void
 /** Reference-counted catalog registration; release when the component unmounts. */
 useCatalog(namespace:string,catalogs:Catalogs):()=>void
 on<K extends keyof HostEvents>(type:K,listener:(payload:HostEvents[K])=>void):()=>void
 emit<K extends keyof HostEvents>(type:K,...payload:HostEvents[K] extends void?[]:[HostEvents[K]]):void
 setSettingsOpener(opener:(appId:string,panelId?:string)=>void):()=>void
 /** Opens the central Settings at an app panel; false when no Settings host is present (standalone). */
 openSettings(appId:string,panelId?:string):boolean
}
export const sdkVersion:number
export function createHost():Host
export function useHost():Host
export function defineSettingsProvider<T extends SettingsProvider>(provider:T):T
