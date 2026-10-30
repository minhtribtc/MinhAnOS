import {IntlMessageFormat} from 'intl-messageformat'
export const locales = ['vi-VN','en-US']
export function resolveLocale(account, guest, languages = []) {
 for (const candidate of [account,guest,...languages]) {
  if (locales.includes(candidate)) return candidate
  // Account/guest values must be exact; language matching is only for navigator hints.
 }
 for(const language of languages){const match=locales.find(l=>l.split('-')[0]===language.split('-')[0]);if(match)return match}
 return 'vi-VN'
}
export function validateCatalog(namespace, catalogs) {
 if(!/^[a-z][a-z0-9-]*$/.test(namespace))throw new Error('Invalid namespace')
 const reference=Object.keys(catalogs['vi-VN']||{}).sort()
 if(!reference.length)throw new Error('Empty catalog')
 const signature=message=>{
  const args=new Set()
  const walk=nodes=>{for(const node of nodes){if(node.value && node.type!==0 && node.type!==7)args.add(node.value+':'+node.type);if(node.options)for(const option of Object.values(node.options))walk(option.value)}}
  walk(new IntlMessageFormat(message,'en-US',undefined,{ignoreTag:true}).getAst())
  return [...args].sort().join(',')
 }
 for(const locale of locales){
  const messages=catalogs[locale]
  if(!messages || JSON.stringify(Object.keys(messages).sort())!==JSON.stringify(reference))throw new Error('Catalog key mismatch: '+locale)
  for(const key of reference){
   if(!key.startsWith(namespace+'.') || !/^[a-zA-Z0-9_.-]+$/.test(key) || typeof messages[key]!=='string')throw new Error('Invalid key: '+key)
   new IntlMessageFormat(messages[key],locale,undefined,{ignoreTag:true})
   if(signature(messages[key])!==signature(catalogs['vi-VN'][key]))throw new Error('Parameter mismatch: '+key)
  }
 }
}
export function createI18n(initial='vi-VN',timeZone='Asia/Bangkok') {
 let locale=resolveLocale(initial), zone=timeZone
 const namespaces=new Map(), cache=new Map()
 return {
  get locale(){return locale},get timeZone(){return zone},
  setLocale(value){if(!locales.includes(value))throw new Error('Unsupported locale');locale=value},
  setTimeZone(value){new Intl.DateTimeFormat(locale,{timeZone:value});zone=value},
  registerCatalog(namespace,catalogs){
   validateCatalog(namespace,catalogs)
   if(namespaces.has(namespace))throw new Error('Namespace already owned: '+namespace)
   const snapshot=structuredClone(catalogs);namespaces.set(namespace,snapshot)
   return ()=>{if(namespaces.get(namespace)===snapshot){namespaces.delete(namespace);cache.clear()}}
  },
  t(key,params={}){
   const catalog=namespaces.get(key.split('.')[0]);const message=catalog?.[locale]?.[key]??catalog?.['vi-VN']?.[key]
   if(message===undefined)return '['+key+']'
   try{const id=locale+'\0'+key;if(!cache.has(id))cache.set(id,new IntlMessageFormat(message,locale,undefined,{ignoreTag:true}));return String(cache.get(id).format(params))}catch{return '['+key+']'}
  },
  formatDate(value,options={}){return new Intl.DateTimeFormat(locale,{timeZone:zone,...options}).format(new Date(value))},
  formatNumber(value,options={}){return new Intl.NumberFormat(locale,options).format(value)},
  formatCurrency(value,currency){return new Intl.NumberFormat(locale,{style:'currency',currency}).format(value)}
 }
}
