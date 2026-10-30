// Drive.Dockerfile copies Go packages one by one. Every internal package the Drive binaries import
// (directly or transitively) must be copied, otherwise the image build fails at deploy time.
// Skips inside image build stages that do not contain Go sources.
import assert from 'node:assert/strict'
import {readFile,readdir} from 'node:fs/promises'
const root=new URL('../',import.meta.url)
const exists=async p=>{try{await readdir(new URL(p,root));return true}catch{return false}}
if(!(await exists('internal/'))||!(await exists('cmd/drive/'))){console.log('SKIP docker context: Go sources not present');process.exit(0)}
const dockerfile=await readFile(new URL('Drive.Dockerfile',root),'utf8')
const imports=async dir=>{const out=new Set();for(const n of await readdir(new URL(dir,root))){if(!n.endsWith('.go')||n.endsWith('_test.go'))continue;for(const m of (await readFile(new URL(dir+n,root),'utf8')).matchAll(/"minhanos\.local\/platform\/(internal\/[a-z0-9_]+)"/g))out.add(m[1])}return out}
const seen=new Set(),queue=['cmd/drive','cmd/files-agent','cmd/identity-admin','cmd/identity-recovery']
while(queue.length){const dir=queue.pop();if(seen.has(dir))continue;seen.add(dir);for(const dep of await imports(dir+'/'))queue.push(dep)}
for(const dir of seen)assert(dockerfile.includes(`COPY ${dir}/ ${dir}/`),`Drive.Dockerfile must COPY ${dir}/`)
console.log('PASS docker context: Drive image copies all '+seen.size+' Go packages it builds')
