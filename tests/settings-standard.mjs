// Central Settings standard (AGENTS.md): every app declares Settings entries in its manifest,
// every entry has a panel in its provider, and every label exists in vi-VN and en-US.
import assert from 'node:assert/strict'
import {readFile,readdir} from 'node:fs/promises'
import {createHost,defineSettingsProvider} from '../packages/app-sdk/src/index.js'
const root=new URL('../',import.meta.url)
const json=async path=>JSON.parse(await readFile(new URL(path,root),'utf8'))
const text=path=>readFile(new URL(path,root),'utf8')

// Catalog owning each nameKey/labelKey namespace, and the source that registers each app's panels.
const catalogs={desktop:'apps/desktop/locales/catalogs.json',accounts:'packages/accounts-app/locales/catalogs.json',files:'packages/files-app/locales/catalogs.json',settings:'packages/settings-app/locales/catalogs.json'}
const providers={'com.minhanos.desktop':'apps/desktop/src/settings.ts','com.minhanos.account-center':'packages/accounts-app/src/settings.ts'}
const registry=await text('apps/desktop/src/settings.ts')

for(const file of (await readdir(new URL('contracts/manifests/',root))).filter(f=>f.endsWith('.app.json'))){
 const manifest=await json('contracts/manifests/'+file)
 assert(Array.isArray(manifest.settingsIntegration),file+': settingsIntegration must be declared (use [] when the app has no settings)')
 assert.equal(typeof manifest.nameKey,'string',file+': nameKey required for the Settings sidebar')
 for(const key of [manifest.nameKey,...manifest.settingsIntegration.map(e=>e.labelKey)]){
  const path=catalogs[key.split('.')[0]];assert(path,file+': no catalog owns '+key)
  const catalog=await json(path);for(const locale of ['vi-VN','en-US'])assert(key in catalog[locale],file+': '+key+' missing in '+locale)
 }
 assert.equal(new Set(manifest.settingsIntegration.map(e=>e.id)).size,manifest.settingsIntegration.length,file+': duplicate settings ids')
 if(manifest.settingsIntegration.length){
  const source=providers[manifest.id];assert(source,file+': declares settings but has no provider')
  const code=await text(source)
  for(const entry of manifest.settingsIntegration)assert(new RegExp(String.raw`\b${entry.id}\s*:`).test(code),file+': panel '+entry.id+' not provided in '+source)
 }
 if(manifest.id!=='com.minhanos.settings')assert(registry.includes(file.replace('.app.json',''))||registry.includes(file),file+': app not registered with the Settings host (apps/desktop/src/settings.ts)')
}
// Settings must not regain hardcoded tabs.
assert(!/settingsTab/.test(await text('apps/desktop/src/components/AppContent.vue')),'Hardcoded Settings tabs found in AppContent.vue')

// SDK: shared catalogs are reference counted, host events and the Settings opener work.
const host=createHost(),demo={'vi-VN':{'demo.x':'X'},'en-US':{'demo.x':'X'}}
const r1=host.useCatalog('demo',demo),r2=host.useCatalog('demo',demo)
assert.throws(()=>host.useCatalog('demo',structuredClone(demo)));r1();r1();assert.equal(host.i18n.t('demo.x'),'X');r2();assert.equal(host.i18n.t('demo.x'),'[demo.x]')
let seen;const off=host.on('profile-changed',p=>{seen=p});host.emit('profile-changed',{displayName:'A'});off();host.emit('profile-changed',{displayName:'B'});assert.equal(seen.displayName,'A')
assert.equal(host.openSettings('com.minhanos.files'),false);let opened;const stop=host.setSettingsOpener((a,p)=>{opened=a+'/'+p});assert(host.openSettings('com.minhanos.desktop','appearance'));assert.equal(opened,'com.minhanos.desktop/appearance');stop();assert.equal(host.openSettings('x'),false)
assert.throws(()=>defineSettingsProvider({manifest:{id:'com.minhanos.x',settingsIntegration:[{id:'a',labelKey:'x.a',path:'/a'}]},panels:{}}))
host.dispose()
console.log('PASS settings standard: manifests declare Settings entries, panels provided, labels in vi/en, no hardcoded tabs, SDK catalog/events/opener')
