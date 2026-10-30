import type {App,DeepReadonly,Ref} from 'vue'
import type {I18n,Locale} from '@minhanos/i18n'
export interface Profile {locale:Locale;theme:'light'|'dark'|'system';timeZone:string}
export type Request=<T=any>(path:string,options?:RequestInit)=>Promise<T>
export class ApiError extends Error {status:number;code:string;requestId?:string;constructor(status:number,body?:{error?:string;code?:string;requestId?:string})}
export function createApiClient(base?:string):{setCSRF(value:string):void;request:Request}
export interface Host {errorMessage(error:unknown):string;version:number;profile:DeepReadonly<Ref<Profile>>;i18n:I18n;install(app:App):void;applyProfile(value:Profile):void;loadProfile(request:Request):Promise<void>;updateProfile(request:Request,patch:Partial<Profile>):Promise<void>;dispose():void}
export const sdkVersion:number
export function createHost():Host
export function useHost():Host
