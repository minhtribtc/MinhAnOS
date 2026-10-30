import type {App,DeepReadonly,Ref} from 'vue'
import type {I18n,Locale} from '@minhanos/i18n'
import type {AccountProfile} from '@minhanos/shared-types'
export type Profile=AccountProfile
export type Request=<T=any>(path:string,options?:RequestInit)=>Promise<T>
export {ApiError,createApiClient} from '@minhanos/api-client'
export interface Host {errorMessage(error:unknown):string;version:number;profile:DeepReadonly<Ref<Profile>>;i18n:I18n;install(app:App):void;applyProfile(value:Profile):void;loadProfile(request:Request):Promise<void>;updateProfile(request:Request,patch:Partial<Profile>):Promise<void>;dispose():void}
export const sdkVersion:number
export function createHost():Host
export function useHost():Host
