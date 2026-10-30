import assert from 'node:assert/strict'
import {readFile,readdir} from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
import {createI18n,resolveLocale,validateCatalog} from '../packages/i18n/src/index.js'
import {ApiError,createApiClient,createHost} from '../packages/app-sdk/src/index.js'
import {resolveTheme} from '../packages/ui/src/index.js'
const root=new URL('../',import.meta.url)
const files=JSON.parse(await readFile(new URL('packages/files-app/locales/catalogs.json',root),'utf8'))
const common=JSON.parse(await readFile(new URL('packages/app-sdk/locales/common.json',root),'utf8'))
const accounts=JSON.parse(await readFile(new URL('packages/accounts-app/locales/catalogs.json',root),'utf8'));validateCatalog('accounts',accounts);
const accountManifest=JSON.parse(await readFile(new URL('contracts/manifests/accounts.app.json',root),'utf8'));for(const item of [...accountManifest.navigation,...accountManifest.settingsIntegration])assert(item.labelKey in accounts['vi-VN']);
validateCatalog('files',files);validateCatalog('common',common)
const a=createI18n('en-US'),b=createI18n('vi-VN');const unregister=a.registerCatalog('files',files);b.registerCatalog('files',files)
assert.equal(a.t('files.actions.copy'),'Copy');assert.equal(b.t('files.actions.copy'),'Sao chép')
assert.equal(a.t('files.selection.count',{count:1}),'1 item selected');assert.equal(a.t('files.selection.count',{count:2}),'2 items selected')
assert.throws(()=>a.registerCatalog('files',files));assert.equal(a.t('missing.key'),'[missing.key]')
unregister();assert.equal(a.t('files.actions.copy'),'[files.actions.copy]');a.registerCatalog('files',files)
const broken=structuredClone(files);delete broken['en-US']['files.actions.copy'];assert.throws(()=>validateCatalog('files',broken))
const mismatch=structuredClone(files);mismatch['en-US']['files.selection.count']='{otherCount}';assert.throws(()=>validateCatalog('files',mismatch))
const badICU=structuredClone(files);badICU['en-US']['files.actions.copy']='{oops';assert.throws(()=>validateCatalog('files',badICU))
assert.equal(resolveLocale('en-US','vi-VN'),'en-US');assert.equal(resolveLocale(undefined,undefined,['en-GB']),'en-US');assert.equal(resolveLocale(undefined,undefined,['ja-JP']),'vi-VN')
assert.equal(a.formatNumber(1234.5),'1,234.5');assert.equal(b.formatNumber(1234.5),'1.234,5');assert.match(a.formatCurrency(10,'USD'),/10/)
a.setTimeZone('UTC');assert.equal(a.formatDate('2026-01-01T23:30:00Z',{hour:'2-digit',hourCycle:'h23'}),'23')
assert.equal(resolveTheme('system',true),'dark');assert.equal(resolveTheme('light',true),'light')
const host=createHost();assert.equal(host.errorMessage(new ApiError(400,{code:'UNKNOWN',error:'sensitive legacy text'})),host.i18n.t('common.errors.REQUEST_FAILED'));host.applyProfile({locale:'future-locale',theme:'system',timeZone:'UTC'});assert.equal(host.profile.value.locale,resolveLocale(undefined,undefined,globalThis.navigator?.languages||[]))
let profileBody
await host.updateProfile(async(path,options)=>{profileBody=JSON.parse(options.body);return profileBody},{theme:'dark'});assert.equal(profileBody.locale,'future-locale')
assert.throws(()=>host.applyProfile(null));host.dispose()
const seen=[];const original=globalThis.fetch
try{globalThis.fetch=async(url,opts)=>{seen.push({url,headers:opts.headers});return{ok:true,json:async()=>({ok:true})}}
const x=createApiClient('/api'),y=createApiClient('/api/drive');x.setCSRF('x');y.setCSRF('y');await x.request('/session');await y.request('/session');assert.equal(seen[0].headers.get('X-CSRF-Token'),'x');assert.equal(seen[1].headers.get('X-CSRF-Token'),'y')
globalThis.fetch=async()=>({ok:false,status:401,json:async()=>({code:'AUTH_REQUIRED',error:'anything'})});await assert.rejects(x.request('/session'),e=>e instanceof ApiError&&e.code==='AUTH_REQUIRED')
}finally{globalThis.fetch=original}
// Enforce architectural boundaries in the normal frontend build. A Drive-only image may omit desktop source.
async function walk(url){let names;try{names=await readdir(url,{withFileTypes:true})}catch(e){if(e.code==='ENOENT')return [];throw e}const result=[];for(const entry of names){if(['node_modules','dist','.git'].includes(entry.name))continue;const next=new URL(entry.name+(entry.isDirectory()?'/':''),url);if(entry.isDirectory())result.push(...await walk(next));else result.push(next)}return result}
for(const dir of ['packages/','apps/accounts/','apps/drive/','internal/drive/','cmd/drive/'])for(const file of await walk(new URL(dir,root))){if(!/\.(vue|ts|js|go)$/.test(file.pathname))continue;const source=await readFile(file,'utf8');assert(!/from\s*['"][^'"]*(?:apps\/desktop|desktop\/src)|internal\/platform(?:["/])/.test(source),'Shell dependency: '+fileURLToPath(file))}
console.log('PASS foundations: ICU/catalogs, provider isolation, locale/Intl, unknown errors, CSRF isolation, theme and dependency boundaries')
