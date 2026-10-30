export type Locale='vi-VN'|'en-US'
export type Catalogs=Record<Locale,Record<string,string>>
export const locales:Locale[]
export function resolveLocale(account?:string,guest?:string,languages?:readonly string[]):Locale
export function validateCatalog(namespace:string,catalogs:Catalogs):void
export interface I18n {
 readonly locale:Locale;readonly timeZone:string
 setLocale(value:Locale):void;setTimeZone(value:string):void
 registerCatalog(namespace:string,catalogs:Catalogs):()=>void
 t(key:string,params?:Record<string,string|number|boolean|Date>):string
 formatDate(value:string|number|Date,options?:Intl.DateTimeFormatOptions):string
 formatNumber(value:number,options?:Intl.NumberFormatOptions):string
 formatCurrency(value:number,currency:string):string
}
export function createI18n(initial?:Locale,timeZone?:string):I18n
