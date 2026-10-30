import Ajv2020 from 'ajv/dist/2020.js'
import {readFile} from 'node:fs/promises'
const schema=JSON.parse(await readFile(new URL('../contracts/apps/catalog.v1.schema.json',import.meta.url),'utf8'))
const check=new Ajv2020({strict:true,allErrors:true}).compile(schema)
export function validateAppCatalog(catalog){
 if(!check(catalog))throw new Error('Invalid app catalog: '+JSON.stringify(check.errors))
 const sets={key:new Set(),moduleName:new Set(),appId:new Set(),alias:new Set()}
 for(const app of catalog.apps){
  for(const field of ['key','moduleName','appId']){if(sets[field].has(app[field]))throw new Error('Duplicate '+field);sets[field].add(app[field])}
  for(const alias of app.legacyRegistryIds){if(sets.alias.has(alias))throw new Error('Ambiguous registry alias');sets.alias.add(alias)}
  if(app.implementation==='planned'&&app.currentPaths.length)throw new Error('Planned app must not claim implementation paths')
  for(const path of app.currentPaths)if(path.split('/').includes('..')||path.includes('//'))throw new Error('Noncanonical app path')
 }
 return catalog
}
