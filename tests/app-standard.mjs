// App standard (docs/app-standard.md) checks for the Desktop Shell, Files and Notes: structure, manifest,
// registry, i18n, theme tokens, SDK, errors, logs, naming and dialog labelling. Static checks only; backend
// rules (audit, quota, policy) have their own Go tests.
import assert from 'node:assert/strict'
import {readFile,readdir} from 'node:fs/promises'
const root=new URL('../',import.meta.url)
const text=path=>readFile(new URL(path,root),'utf8')
const json=async path=>JSON.parse(await text(path))
async function files(dir,ext){const out=[];for(const e of await readdir(new URL(dir,root),{withFileTypes:true})){const p=dir+'/'+e.name;if(e.isDirectory())out.push(...await files(p,ext));else if(ext.some(x=>e.name.endsWith(x)))out.push(p)}return out}

const apps={desktop:{src:['apps/desktop/src'],catalog:'apps/desktop/locales/catalogs.json',manifest:'contracts/manifests/desktop.app.json'},
 files:{src:['packages/files-app/src','apps/drive/src'],catalog:'packages/files-app/locales/catalogs.json',manifest:'contracts/manifests/files.app.json'},
 notes:{src:['packages/notes-app/src'],catalog:'packages/notes-app/locales/catalogs.json',manifest:'contracts/manifests/notes.app.json'}}
const allCatalogs={}
for(const dir of ['accounts','files','settings','storage','notifications','logs','notes'])Object.assign(allCatalogs,(await json(`packages/${dir}-app/locales/catalogs.json`))['vi-VN'])
Object.assign(allCatalogs,(await json('apps/desktop/locales/catalogs.json'))['vi-VN'],(await json('packages/app-sdk/locales/common.json'))['vi-VN'])
// Language names are written in their own language (endonyms) and are not translated.
const endonyms=['Tiếng Việt']

for(const [name,app] of Object.entries(apps)){
 const sources=(await Promise.all(app.src.map(d=>files(d,['.vue','.ts','.css'])))).flat()
 const catalog=await json(app.catalog)
 // Row 5: vi-VN/en-US parity, and every literal key used in code exists.
 assert.deepEqual(Object.keys(catalog['vi-VN']).sort(),Object.keys(catalog['en-US']).sort(),name+': vi-VN/en-US keys differ')
 for(const file of sources){
  const code=await text(file)
  if(file.endsWith('.css')){
   // Row 6: colours only from @minhanos/ui tokens.
   assert(!/#[0-9a-fA-F]{3,8}\b/.test(code),file+': hex colour; use @minhanos/ui tokens')
   assert(!/\brgba?\(\s*\d/.test(code)&&!/\bhsla?\(\s*\d/.test(code),file+': literal colour function; use tokens')
   continue
  }
  let stripped=code.replace(/\/\/[^\n]*|\/\*[\s\S]*?\*\//g,'')
  for(const e of endonyms)stripped=stripped.replaceAll(e,'')
  assert(!/[À-ỹ]/.test(stripped),file+': Vietnamese UI literal; use a catalog key')
  assert(!/#[0-9a-fA-F]{6}\b/.test(stripped),file+': hex colour in a component')
  for(const [,key] of stripped.matchAll(/\bt\(\s*'([a-z][A-Za-z0-9_.]*\.[A-Za-z0-9_]+)'/g))assert(key in allCatalogs,file+': missing catalog key '+key)
  // Row 7: catalogs through host.useCatalog (reference counted), never registerCatalog directly.
  assert(!/registerCatalog\(/.test(code),file+': use host.useCatalog')
  // Row 8: the UI never shows server prose.
  assert(!/\b(e|err|error)\.message\b/.test(stripped),file+': shows an error message; map the error code')
  // Row 23: no legacy or preview names.
  assert(!/MinhAnOS Drive|File Station|Preview \d|PREVIEW/.test(stripped),file+': legacy or preview name')
  // Row 22: dialogs are modal and labelled.
  for(const [tag] of stripped.matchAll(/<[a-z]+[^>]*role="dialog"[^>]*>/g))assert(/aria-modal="true"/.test(tag)&&/aria-label(ledby)?=/.test(tag),file+': dialog needs aria-modal and a label: '+tag.slice(0,80))
  // Row 1: app packages do not import a Shell.
  if(!file.startsWith('apps/desktop'))assert(!/from '(\.\.\/)+(apps\/desktop|store)'|apps\/desktop\/src/.test(code),file+': imports the Shell')
 }
 // Row 2: manifest with real entrypoint, name, navigation, lifecycle and settings entries.
 const manifest=await json(app.manifest)
 for(const field of ['entrypoint','nameKey','navigation','lifecycle','settingsIntegration'])assert(field in manifest,app.manifest+': '+field+' missing')
 for(const item of manifest.navigation)assert(item.labelKey in catalog['vi-VN'],app.manifest+': '+item.labelKey+' missing in catalog')
 assert(manifest.nameKey in catalog['vi-VN'])
}

// Row 3: the launcher reads names from manifests; the backend registry carries IDs, icons and accents only.
const registry=await text('internal/platform/server.go')
const block=registry.slice(registry.indexOf('var registry = []AppDefinition{'),registry.indexOf('}\n',registry.indexOf('var registry = []AppDefinition{')))
assert(!/[À-ỹ]/.test(block)&&!/Title|Subtitle/.test(registry.slice(registry.indexOf('type AppDefinition struct'),registry.indexOf('var registry'))),'backend registry must not carry display names')
const shellRegistry=await text('apps/desktop/src/registry.ts')
for(const [,id] of block.matchAll(/\{"([a-z.-]+)",/g)){
 const catalog=await json('contracts/apps/catalog.v1.json'),entry=catalog.apps.find(a=>a.appId===id||a.legacyRegistryIds.includes(id))
 assert(entry,'registry ID not in app catalog: '+id)
 assert(shellRegistry.includes(`manifests/${entry.key==='accounts'?'accounts':entry.key}.app.json`),'Shell registry lacks the manifest of '+id)
 assert(entry.key+'.app.name' in allCatalogs||(entry.key==='accounts'&&'accounts.app.name' in allCatalogs),'launcher name missing for '+id)
}
// Row 1: no built-in app screens inside the Shell.
const content=await text('apps/desktop/src/components/AppContent.vue')
assert(!/class="(standard-content|overview-layout|notes-content)"/.test(content),'built-in app screens are back in AppContent.vue')

// Rows 8 and 12 (Files backend): every error has a stable code; operational logs are structured.
for(const file of (await files('internal/drive',['.go'])).filter(f=>!f.endsWith('_test.go'))){
 const code=await text(file)
 for(const [call] of code.matchAll(/\bfail\(w, [^\n]*\)/g))assert(/, "[A-Z][A-Z0-9_]+"\)$/.test(call)||/(fe|f)\.Code\)$|, code\)$/.test(call),file+': error without a code: '+call)
 for(const [call] of code.matchAll(/\bproblem\([^\n]*?\)/g))if(!call.startsWith('problem(status'))assert(/^problem\(\d+, "[A-Z][A-Z0-9_]+", "/.test(call),file+': file error without a code: '+call)
 assert(!/\blog\.Print/.test(code),file+': use the structured logger (logError/logInfo)')
}
for(const file of ['internal/platform/server.go','internal/platform/storage.go'])assert(!/\blog\.Print/.test(await text(file)),file+': unstructured log')
// Row 8 (Desktop backend): legacy /api errors carry a stable code too.
for(const file of (await files('internal/platform',['.go'])).filter(f=>!f.endsWith('_test.go'))){
 for(const [call] of (await text(file)).matchAll(/\bwriteError\(w, [^()\n]*\)/g))assert(/, "[A-Z][A-Z0-9_]+"\)$|\.Code\)$/.test(call),file+': error without a code: '+call)
}
// Row 10: the legacy free-text audit tables are no longer written.
assert(!/INSERT INTO (drive_)?audit\(/.test(registry+await text('internal/drive/auth.go')),'legacy audit write')
console.log('PASS app standard: Desktop, Files, Notes — manifests, registry, i18n parity/keys, tokens only, useCatalog, error codes, structured logs, naming, labelled dialogs')
