import {readFile} from 'node:fs/promises'
export function assertAppCatalogCompatibility(previous,next){
 const byId=new Map(next.apps.map(app=>[app.appId,app]))
 for(const old of previous.apps){
  const current=byId.get(old.appId)
  if(!current)throw new Error('Stable app identity removed: '+old.appId)
  for(const alias of old.legacyRegistryIds)if(!current.legacyRegistryIds.includes(alias))throw new Error('Legacy alias removed or reassigned: '+alias)
 }
}
if(process.argv[1]?.replaceAll('\\','/').endsWith('/app-catalog-compatibility.mjs')){
 const previous=JSON.parse(await readFile(process.argv[2],'utf8')),next=JSON.parse(await readFile(process.argv[3],'utf8'))
 assertAppCatalogCompatibility(previous,next);console.log('PASS stable App IDs and legacy alias ownership preserved')
}
