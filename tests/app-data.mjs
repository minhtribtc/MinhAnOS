// App data contracts (app standard rows 17 and 19): every table an app creates is listed with its retention and
// what happens when the account is deleted, removal code touches every table it claims to clean, the export
// operation is documented, and the backup list names a restore test that exists.
import assert from 'node:assert/strict'
import {access,readFile,readdir} from 'node:fs/promises'
import Ajv2020 from 'ajv/dist/2020.js'
const root=new URL('../',import.meta.url),text=file=>readFile(new URL(file,root),'utf8'),json=async file=>JSON.parse(await text(file))
const ajv=new Ajv2020({strict:true,allErrors:true}),valid=ajv.compile(await json('contracts/data/app-data.v1.schema.json'))
const api=await json('contracts/api/openapi.yaml')
const created=sql=>[...sql.matchAll(/CREATE TABLE (?:IF NOT EXISTS )?([a-z_]+)/g)].map(m=>m[1])
const platform=await text('internal/platform/server.go'),schema=platform.slice(platform.indexOf('const schema'),platform.indexOf('`\n',platform.indexOf('const schema')+20))
const driveSQL=(await Promise.all((await readdir(new URL('internal/drive/migrations/',root))).map(f=>text('internal/drive/migrations/'+f)))).join('\n')
// users and sessions in the legacy Desktop schema belong to identity (identity module owns and cleans them).
const apps={desktop:{tables:created(schema).filter(t=>!['users','sessions'].includes(t)),module:'desktop'},files:{tables:created(driveSQL),module:'files'}}
for(const [name,app] of Object.entries(apps)){
 const data=await json(`contracts/data/${name}.data.json`)
 assert(valid(data),name+': '+ajv.errorsText(valid.errors))
 assert.deepEqual(data.tables.map(t=>t.name).sort(),[...app.tables].sort(),name+': data contract lists exactly the tables the app creates')
 for(const t of data.tables)assert(t.personal!==(t.onAccountDelete==='not-personal'||t.onAccountDelete==='kept-by-retention'),name+'.'+t.name+': personal rows must be removed on account delete')
 const [file,fn]=data.removal.function.split(':'),code=await text(file)
 assert(new RegExp(`func (\\([^)]*\\) )?${fn}\\(`).test(code),name+': removal function missing')
 for(const t of data.tables.filter(t=>t.onAccountDelete.startsWith('removed')))assert(code.includes(t.name),name+': removal does not touch '+t.name)
 const ops=Object.values(api.paths).flatMap(item=>Object.values(item))
 const op=ops.find(o=>o.operationId===data.export.operationId)
 assert(op&&op['x-module']===app.module,name+': export operation not in OpenAPI')
 assert(op.responses['200'].content[data.export.format],name+': export format differs from OpenAPI')
 await access(new URL(data.restoreTest,root))
}
const files=await json('contracts/data/files.data.json'),userdata=await text('internal/drive/userdata.go')
assert(userdata.includes('RemovalGraceDays = '+files.removal.graceDays),'grace period differs from the Go constant')
const restore=await text('tests/backup-restore.sh')
assert(restore.includes('contracts/data/')&&restore.includes('--network none'),'restore test checks the listed tables in an isolated database')
console.log('PASS app data: tables, retention and account-delete rules listed, removal code, export in OpenAPI, restore test present')
