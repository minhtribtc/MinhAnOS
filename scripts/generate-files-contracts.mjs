// Writes contracts/files-api and contracts/desktop schemas and their OpenAPI operations (x-module files/desktop).
// Run after changing a Files or Desktop route or DTO: node scripts/generate-files-contracts.mjs . && npm run contracts:generate
import {readFile,writeFile,mkdir} from 'node:fs/promises'
const root=process.argv[2]||'.'
const S='https://json-schema.org/draft/2020-12/schema'
const obj=(properties,required=Object.keys(properties))=>({type:'object',additionalProperties:false,properties,required})
const str=(extra={})=>({type:'string',...extra})
const int=(extra={})=>({type:'integer',...extra})
const time={type:'string',format:'date-time'}
const entry=obj({name:str({minLength:1}),path:str(),kind:{enum:['file','folder','symlink','special']},size:int({minimum:0}),modified:time,mode:str(),uid:int({minimum:0}),gid:int({minimum:0}),etag:str()})
const trashItem=obj({id:str({pattern:'^[A-Za-z0-9_-]{8,128}$'}),path:str(),name:str(),deleted:time})
const usage=obj({limitBytes:{type:['integer','null'],minimum:0},usedBytes:int({minimum:0}),reservedBytes:int({minimum:0}),measured:{type:'boolean'}})
const user=obj({id:str(),username:str({pattern:'^[a-z][a-z0-9_-]{2,31}$'}),displayName:str(),role:{enum:['admin','user']}})
const session=obj({user,csrfToken:str({minLength:16}),passwordChangeRequired:{type:'boolean'}})
// A Files job is a contracts/jobs job.v1 narrowed to the copy type and byte progress.
const job=JSON.parse(await readFile(`${root}/contracts/jobs/job.v1.schema.json`,'utf8'));delete job.$schema;delete job.$id
job.properties.id={type:'string',format:'uuid'};job.properties.type={const:'files.copy'};job.properties.progress.properties.unit={const:'bytes'};job.properties.maxAttempts={type:'integer',minimum:1,maximum:10}
const files={
 FilesEntry:entry,
 FilesJob:job,
 FilesJobList:obj({items:{type:'array',maxItems:20,items:job}}),
 FilesListPage:obj({path:str(),entries:{type:'array',maxItems:1000,items:entry},nextCursor:{type:['string','null'],pattern:'^[A-Za-z0-9_-]+$'}}),
 FilesSearchResult:obj({path:str(),entries:{type:'array',maxItems:1000,items:entry},truncated:{type:'boolean'}}),
 FilesText:obj({content:str({maxLength:1048576}),etag:str()}),
 FilesTrashItem:trashItem,
 FilesTrashList:{type:'array',maxItems:10000,items:trashItem},
 FilesNameInput:obj({name:str({minLength:1,maxLength:255})}),
 FilesDestinationInput:obj({destination:str({minLength:1,maxLength:4096})}),
 FilesRestoreInput:obj({id:str({pattern:'^[A-Za-z0-9_-]{8,128}$'})}),
 FilesTrashDeleteInput:obj({id:str({pattern:'^[A-Za-z0-9_-]{8,128}$'})}),
 FilesRemoveInput:obj({confirm:{const:true}}),
 FilesSaveTextInput:obj({content:str({maxLength:1048576})}),
 FilesWriteResult:obj({ok:{const:true},item:{anyOf:[entry,trashItem]},backup:str(),job},['ok']),
 FilesRootList:obj({roots:{type:'array',maxItems:1000,items:obj({id:str({pattern:'^(personal|system|share:[a-z0-9][a-z0-9-]{0,62})$'}),kind:{enum:['home','share','system']},name:str({minLength:1,maxLength:100}),access:{enum:['read','write']},organizationId:str({format:'uuid'}),organization:str({minLength:1,maxLength:100})},['id','kind','access'])},homes:{type:'boolean'},maxUploadBytes:int({minimum:1}),maxTextBytes:int({minimum:1}),quota:usage,sharing:{type:'boolean'},publicBase:str({maxLength:2048}),publicPort:str({maxLength:5}),activityLevel:{enum:['minimal','writes','access','full']}},['roots','homes','maxUploadBytes','maxTextBytes']),
 FilesQuotaFolderList:obj({items:{type:'array',maxItems:1000,items:obj({id:str({format:'uuid'}),shareId:str({format:'uuid'}),path:str(),version:int({minimum:1}),usage})}}),
 FilesSession:session,
 FilesLoginInput:obj({username:str({minLength:1,maxLength:64}),password:str({minLength:1,maxLength:256})}),
 FilesOk:obj({ok:{const:true}}),
 FilesActivitySettings:obj({level:{enum:['minimal','writes','access','full']},version:int({minimum:1}),retentionDays:int({minimum:1})},['level','version']),
 FilesActivityLevelInput:obj({level:{enum:['minimal','writes','access','full']}},['level']),
 FilesPresenceInput:obj({root:str({maxLength:200}),rootName:str({maxLength:200}),path:str({maxLength:4096}),file:str({maxLength:4096})},['root','path']),
 FilesPresenceResult:obj({stored:{type:'boolean'}},['stored']),
 FilesLiveList:obj({level:{enum:['minimal','writes','access','full']},items:{type:'array',maxItems:5000,items:obj({user:str({maxLength:100}),displayName:str({maxLength:200}),kind:{enum:['view','open','download','archive','upload']},root:str({maxLength:200}),rootName:str({maxLength:200}),path:str({maxLength:4096}),file:str({maxLength:4096}),since:str({format:'date-time'}),seen:str({format:'date-time'}),client:str({maxLength:100}),agent:str({maxLength:200}),bytes:int({minimum:0}),total:int({minimum:0})},['user','kind','root','path','since','seen'])}},['level','items']),
 FilesPurgeResult:obj({ok:{const:true},freedBytes:int({minimum:0})}),
 FilesHealth:obj({app:{const:'MinhAnOS Files'},version:str(),independent:{type:'boolean'}}),
}
const win=obj({id:str(),x:{type:'number',minimum:0,maximum:20000},y:{type:'number',minimum:0,maximum:20000},width:{type:'number',minimum:260,maximum:10000},height:{type:'number',minimum:200,maximum:10000},minimized:{type:'boolean'},maximized:{type:'boolean'}})
const prefs=obj({theme:{enum:['light','dark']},wallpaper:{enum:['mountain','midnight','sage']},mode:{enum:['focus','windows']},windows:{type:'array',maxItems:32,items:win},notes:str({maxLength:32000}),pinned:{type:'array',maxItems:32,items:str()}})
const desktop={
 DesktopSession:session,
 DesktopLoginInput:obj({username:str({minLength:1,maxLength:64}),password:str({minLength:1,maxLength:256})}),
 DesktopAppList:{type:'array',maxItems:64,items:obj({id:str({pattern:'^[a-z][a-z0-9.-]*$'}),icon:str({pattern:'^[a-z][a-z0-9-]*$'}),color:str({pattern:'^[a-z]+$'})})},
 DesktopPreferences:prefs,
 DesktopExport:obj({app:{const:'com.minhanos.desktop'},exportVersion:{const:'1'},exportedAt:time,preferences:prefs}),
 DesktopSystemStatus:obj({version:str(),hostname:str(),os:str(),architecture:str(),goVersion:str(),cpus:int({minimum:1}),goroutines:int({minimum:0}),heapBytes:int({minimum:0}),uptimeSeconds:int({minimum:0}),diskTotalBytes:int({minimum:0}),diskFreeBytes:int({minimum:0}),database:{enum:['connected','disconnected']}}),
 DesktopOk:obj({ok:{const:true}}),
 DesktopHealth:obj({status:{const:'ok'},version:str()}),
 DesktopSetupStatus:obj({required:{type:'boolean'}}),
 DesktopSetupInput:obj({setupCode:str({minLength:1,maxLength:256}),username:str({pattern:'^[a-z][a-z0-9_-]{2,31}$'}),displayName:str({minLength:1,maxLength:100}),password:str({minLength:12,maxLength:256}),organizationName:str({minLength:1,maxLength:100})}),
 DesktopSetupResult:obj({user:obj({username:str(),displayName:str()}),organization:str()}),
}
const fileName=type=>type.replace(/^(Files|Desktop)/,'').replace(/[A-Z]/g,(c,i)=>(i?'-':'')+c.toLowerCase())+'.v1.schema.json'
async function writeGroup(dir,id,group,implemented){
 await mkdir(`${root}/contracts/${dir}`,{recursive:true})
 const index={contractVersion:'1',schemas:[]}
 for(const [type,schema] of Object.entries(group)){const file=fileName(type);index.schemas.push({file,type});await writeFile(`${root}/contracts/${dir}/${file}`,JSON.stringify({$schema:S,$id:`https://schemas.minhanos.local/${id}-${file.replace('.v1.schema.json','')}.v1`,...schema},null,2)+'\n')}
 await writeFile(`${root}/contracts/${dir}/index.v1.json`,JSON.stringify(index,null,2)+'\n')
 await writeFile(`${root}/contracts/${dir}/status.v1.json`,JSON.stringify({contractVersion:'1',implementedOperations:implemented},null,2)+'\n')
}
const ref=type=>({$ref:'#/components/schemas/'+type})
const json=type=>({'application/json':{schema:ref(type)}})
const error={description:'Request failed (stable code; the message is not shown to users)',content:json('LegacyError')}
const header=(name,description,schema={type:'string'})=>({name,in:'header',required:true,description,schema})
const csrf=header('X-CSRF-Token','CSRF token from the session')
const idem=header('Idempotency-Key','16–128 characters; a retry with the same key and request replays the response for 24 hours',{type:'string',minLength:16,maxLength:128})
const ifMatch=header('If-Match','ETag of the version the client last read',{type:'string',minLength:1})
const q=(name,description,schema={type:'string'},required=false)=>({name,in:'query',required,description,schema})
const rootQ=q('root','personal, share:<key>, shared:<grant id> (an item shared with the user) or system',{type:'string',pattern:'^(personal|system|share:[a-z0-9][a-z0-9-]{0,62}|shared:[0-9a-f]{64})$'})
const favorite=obj({root:{type:'string',pattern:rootQ.schema.pattern},path:str({minLength:1,maxLength:4096}),dir:{type:'boolean'},created:time})
files.FilesFavoriteList=obj({items:{type:'array',maxItems:500,items:favorite},limit:int({minimum:1})})
files.FilesFavoriteInput=obj({root:{type:'string',pattern:rootQ.schema.pattern},path:str({minLength:1,maxLength:4096}),dir:{type:'boolean'}},['root','path'])
const pathQ=q('path','Path relative to the root',{type:'string',maxLength:4096})
// Sharing (ADR 0011): grants to people of a common organization and public links; IDs and tokens are 64 hex.
const hex64=str({pattern:'^[0-9a-f]{64}$'})
const grant=obj({id:hex64,path:str({minLength:1,maxLength:4096}),name:str(),dir:{type:'boolean'},permission:{enum:['view','edit']},owner:str(),granteeId:str({format:'uuid'}),grantee:str(),created:time})
const link=obj({id:hex64,token:hex64,path:str({minLength:1,maxLength:4096}),name:str(),dir:{type:'boolean'},permission:{enum:['view','edit','drop']},hasPassword:{type:'boolean'},expires:{type:['string','null'],format:'date-time'},created:time,lastUsed:{type:['string','null'],format:'date-time'},uses:int({minimum:0})})
files.FilesGrant=grant
files.FilesLink=link
files.FilesSharingList=obj({grants:{type:'array',maxItems:1000,items:grant},links:{type:'array',maxItems:500,items:link},limits:obj({grants:int({minimum:1}),links:int({minimum:1})})})
files.FilesReceivedList=obj({items:{type:'array',maxItems:10000,items:grant}})
files.FilesRecipientList=obj({items:{type:'array',maxItems:20,items:obj({id:str({format:'uuid'}),username:str(),displayName:str()})}})
files.FilesGrantInput=obj({path:str({minLength:1,maxLength:4096}),recipient:str({format:'uuid'}),permission:{enum:['view','edit']}})
files.FilesLinkInput=obj({path:str({minLength:1,maxLength:4096}),permission:{enum:['view','edit','drop']},password:str({maxLength:256}),expires:str({maxLength:64})},['path','permission'])
files.FilesLinkUpdateInput=obj({id:hex64,permission:{enum:['view','edit','drop']},expires:str({maxLength:64}),clearPassword:{type:'boolean'},password:str({maxLength:256})},['id','permission'])
files.FilesIdInput=obj({id:hex64})
files.FilesPublicInfo=obj({name:str(),dir:{type:'boolean'},permission:{enum:['view','edit','drop']},owner:str(),needsPassword:{type:'boolean'},expires:{type:['string','null'],format:'date-time'},maxUploadBytes:int({minimum:1})})
files.FilesUnlockInput=obj({password:str({minLength:1,maxLength:256})})
const tokenP={name:'token',in:'path',required:true,description:'Secret token of the public link',schema:hex64}
const linkPathQ=q('path','Path relative to the shared item',{type:'string',maxLength:4096})
const jobID={name:'id',in:'path',required:true,description:'Job ID',schema:{type:'string',format:'uuid'}}
const ops={}
const op=(path,method,operationId,summary,{params=[],body,ok='200',response,security='driveSession',module,extra={}}={})=>{
 const o={operationId,summary,'x-implementation':'implemented','x-module':module,tags:[module],parameters:params,responses:{[ok]:{description:'Successful response',content:response==='binary'?{'application/octet-stream':{schema:{type:'string',format:'binary'}}}:response==='tar'?{'application/x-tar':{schema:{type:'string',format:'binary'}}}:response==='zip'?{'application/zip':{schema:{type:'string',format:'binary'}}}:json(response)},default:error},...extra}
 if(security)o.security=[{[security]:[]}];else o.security=[]
 if(body)o.requestBody={required:true,content:body==='binary'?{'application/octet-stream':{schema:{type:'string',format:'binary'}}}:json(body)}
 if(!o.parameters.length)delete o.parameters
 ;(ops[path]??={})[method]=o
 return operationId
}
const F='/api/drive',fm={module:'files'}
const w=(name,id,summary,body,response='FilesWriteResult',extra={})=>op(`${F}/files/${name}`,'post',id,summary,{...fm,params:[csrf,idem,rootQ,pathQ,...(extra.params||[])],body,response,ok:extra.ok||'200',extra:extra.op||{}})
const filesOps=[
 op(`${F}/health`,'get','filesHealth','Files service health',{...fm,security:null,response:'FilesHealth'}),
 op(`${F}/login`,'post','filesLogin','Sign in to the standalone Files origin (port 8082) without the Desktop',{...fm,security:null,body:'FilesLoginInput',response:'FilesSession'}),
 op(`${F}/session`,'get','filesSession','Current Files session and CSRF token',{...fm,response:'FilesSession'}),
 op(`${F}/logout`,'post','filesLogout','Sign out of the standalone Files origin (refused for Desktop sessions)',{...fm,params:[csrf],response:'FilesOk'}),
 op(`${F}/roots`,'get','filesRoots','Storage roots the user may open, limits and home quota',{...fm,response:'FilesRootList'}),
 op(`${F}/quota-folders`,'get','filesQuotaFolders','Quota folders of a Shared Folder the user may read',{...fm,params:[rootQ],response:'FilesQuotaFolderList'}),
 op(`${F}/files/list`,'get','filesList','One page of a folder listing (folders first, then by name)',{...fm,params:[rootQ,pathQ,q('limit','1–1000, default 1000',{type:'integer',minimum:1,maximum:1000}),q('cursor','nextCursor of the previous page',{type:'string',pattern:'^[A-Za-z0-9_-]+$'})],response:'FilesListPage'}),
 op(`${F}/files/search`,'get','filesSearch','Search names under path (case-insensitive; at most 200 results, truncated when the walk stopped early)',{...fm,params:[rootQ,pathQ,q('q','Name contains (case-insensitive), 1–255 characters',{type:'string',minLength:1,maxLength:255},true)],response:'FilesSearchResult'}),
 op(`${F}/files/recent`,'get','filesRecent','The 50 most recently modified regular files under path',{...fm,params:[rootQ,pathQ],response:'FilesSearchResult'}),
 op(`${F}/files/download`,'get','filesDownload','Download a file (Content-Disposition attachment)',{...fm,params:[rootQ,pathQ],response:'binary'}),
 op(`${F}/files/archive`,'get','filesArchive','Download the selected items as a zip archive',{...fm,params:[rootQ,{...q('item','Path of an item to include; repeat for several (at most 1000)',{type:'array',items:{type:'string',maxLength:4096},minItems:1,maxItems:1000},true),style:'form',explode:true}],response:'zip'}),
 op(`${F}/files/text`,'get','filesText','Read a text file up to maxTextBytes with its ETag',{...fm,params:[rootQ,pathQ],response:'FilesText'}),
 op(`${F}/files/trash`,'get','filesTrashList','Items in the trash of the root',{...fm,params:[rootQ],response:'FilesTrashList'}),
 w('upload','filesUpload','Upload a file (body is the raw content, at most maxUploadBytes)','binary','FilesEntry',{ok:'201',params:[q('name','File name',{type:'string',minLength:1,maxLength:255},true)]}),
 w('mkdir','filesMkdir','Create a folder','FilesNameInput'),
 w('rename','filesRename','Rename a file or folder in place','FilesNameInput'),
 w('move','filesMove','Move to another path in the same root','FilesDestinationInput'),
 w('copy','filesCopy','Copy to another path in the same root; a folder or a file over maxUploadBytes is queued as a job (202)','FilesDestinationInput'),
 w('trash','filesTrash','Move to the trash of the root','FilesEmptyInput'),
 w('restore','filesRestore','Restore a trash item','FilesRestoreInput'),
 w('empty-trash','filesEmptyTrash','Empty the trash of the root (personal and share roots only)','FilesEmptyInput','FilesPurgeResult'),
 w('trash-delete','filesTrashDelete','Delete one item from the trash permanently','FilesTrashDeleteInput','FilesPurgeResult'),
 w('remove','filesRemove','Delete permanently (system root only, needs confirm)','FilesRemoveInput'),
 w('save-text','filesSaveText','Replace a text file; If-Match carries its ETag, the previous content is kept as a backup','FilesSaveTextInput','FilesWriteResult',{params:[ifMatch]}),
 op(`${F}/export`,'get','filesExport','Download the personal home as a tar archive (folders and regular files; trash, links and special files excluded)',{...fm,response:'tar'}),
 op(`${F}/jobs`,'get','filesJobs','The user\'s 20 most recent Files jobs (finished jobs are kept 7 days)',{...fm,response:'FilesJobList'}),
 op(`${F}/jobs/{id}`,'get','filesJob','One Files job with its progress',{...fm,params:[jobID],response:'FilesJob'}),
 op(`${F}/jobs/{id}/cancel`,'post','filesCancelJob','Request cancel (idempotent; a running job stops at its next progress report)',{...fm,params:[csrf,jobID],response:'FilesJob'}),
 op(`${F}/favorites`,'get','filesFavorites','Favorites the user can still open',{...fm,response:'FilesFavoriteList'}),
 op(`${F}/favorites`,'post','filesAddFavorite','Add a favorite (409 FILES_FAVORITES_LIMIT at 500)',{...fm,params:[csrf],body:'FilesFavoriteInput',response:'FilesOk'}),
 op(`${F}/favorites/remove`,'post','filesRemoveFavorite','Remove a favorite',{...fm,params:[csrf],body:'FilesFavoriteInput',response:'FilesOk'}),
 op(`${F}/sharing`,'get','filesSharing','Grants and public links of the personal home (of one item with path)',{...fm,params:[q('path','Only the shares of this item',{type:'string',maxLength:4096})],response:'FilesSharingList'}),
 op(`${F}/sharing/received`,'get','filesSharingReceived','Items shared with the user by people of a common active organization (open as root shared:<id>)',{...fm,response:'FilesReceivedList'}),
 op(`${F}/sharing/recipients`,'get','filesSharingRecipients','Up to 20 people the user may share with (same active organization)',{...fm,params:[q('q','Name or username contains',{type:'string',maxLength:100})],response:'FilesRecipientList'}),
 op(`${F}/sharing/grants`,'post','filesShareWith','Share an item of the home with a person, or change the permission (edit on a file allows saving its text only)',{...fm,params:[csrf,idem],body:'FilesGrantInput',response:'FilesGrant'}),
 op(`${F}/sharing/grants/remove`,'post','filesUnshare','Stop sharing with a person (their favorites of it go too)',{...fm,params:[csrf,idem],body:'FilesIdInput',response:'FilesOk'}),
 op(`${F}/sharing/links`,'post','filesCreateLink','Create a public link: view; for folders also edit (upload, new folder) or drop (upload only); optional password (6–256) and expiry',{...fm,params:[csrf,idem],body:'FilesLinkInput',response:'FilesLink'}),
 op(`${F}/sharing/links/update`,'post','filesUpdateLink','Change permission, expiry or password of a public link',{...fm,params:[csrf,idem],body:'FilesLinkUpdateInput',response:'FilesLink'}),
 op(`${F}/sharing/links/remove`,'post','filesRemoveLink','Delete a public link',{...fm,params:[csrf,idem],body:'FilesIdInput',response:'FilesOk'}),
 op(`${F}/activity/presence`,'post','filesActivityPresence','Report the folder or file open in the app (kept in memory at level full only)',{...fm,params:[csrf],body:'FilesPresenceInput',response:'FilesPresenceResult'}),
 op(`${F}/activity/live`,'get','filesActivityLive','Who has what open and the transfers running (System Admin; empty below level full)',{...fm,response:'FilesLiveList'}),
 op(`${F}/activity/settings`,'get','filesActivitySettings','Files activity log level (System Admin); ETag is the version',{...fm,response:'FilesActivitySettings'}),
 op(`${F}/activity/settings`,'put','filesSetActivitySettings','Change the activity log level (System Admin; If-Match the ETag, 412 VERSION_CONFLICT otherwise); the change is audited',{...fm,params:[csrf,ifMatch],body:'FilesActivityLevelInput',response:'FilesActivitySettings'}),
 op(`${F}/public/{token}`,'get','filesPublicInfo','What a public link shares (no account; 404 FILES_LINK_NOT_FOUND, 410 FILES_LINK_EXPIRED)',{...fm,security:null,params:[tokenP],response:'FilesPublicInfo'}),
 op(`${F}/public/{token}/unlock`,'post','filesPublicUnlock','Unlock a password-protected link for 12 hours (cookie; 403 FILES_LINK_PASSWORD_WRONG, 429 after 10 tries)',{...fm,security:null,params:[tokenP],body:'FilesUnlockInput',response:'FilesOk'}),
 op(`${F}/public/{token}/files/list`,'get','filesPublicList','One page of a folder of the link (view, edit)',{...fm,security:null,params:[tokenP,linkPathQ,q('cursor','nextCursor of the previous page',{type:'string',pattern:'^[A-Za-z0-9_-]+$'})],response:'FilesListPage'}),
 op(`${F}/public/{token}/files/download`,'get','filesPublicDownload','Download a file of the link (view, edit)',{...fm,security:null,params:[tokenP,linkPathQ],response:'binary'}),
 op(`${F}/public/{token}/files/archive`,'get','filesPublicArchive','Download items of the link as a zip archive (view, edit)',{...fm,security:null,params:[tokenP,{...q('item','Path of an item to include; repeat for several (at most 1000)',{type:'array',items:{type:'string',maxLength:4096},minItems:1,maxItems:1000},true),style:'form',explode:true}],response:'zip'}),
 op(`${F}/public/{token}/files/upload`,'post','filesPublicUpload','Upload through the link (edit; drop at the top folder only); counts against the owner quota',{...fm,security:null,params:[tokenP,linkPathQ,q('name','File name',{type:'string',minLength:1,maxLength:255},true)],body:'binary',response:'FilesEntry',ok:'201'}),
 op(`${F}/public/{token}/files/mkdir`,'post','filesPublicMkdir','Create a folder through the link (edit)',{...fm,security:null,params:[tokenP,linkPathQ],body:'FilesNameInput',response:'FilesWriteResult'}),
]
ops[`${F}/files/copy`].post.responses['202']={description:'Queued as a background job (job holds its id and progress)',content:json('FilesWriteResult')}
ops[`${F}/favorites`].post.responses['409']={description:'FILES_FAVORITES_LIMIT: the user already has 500 favorites',content:json('LegacyError')}
files.FilesEmptyInput=obj({},[])
const D='/api',dm={module:'desktop',security:'desktopSession'}
const desktopOps=[
 op(`${D}/health`,'get','desktopHealth','Desktop service health',{...dm,security:null,response:'DesktopHealth'}),
 op(`${D}/login`,'post','desktopLogin','Sign in to the Desktop',{...dm,security:null,body:'DesktopLoginInput',response:'DesktopSession'}),
 op(`${D}/session`,'get','desktopSession','Current Desktop session and CSRF token',{...dm,response:'DesktopSession'}),
 op(`${D}/logout`,'post','desktopLogout','Sign out and revoke the session',{...dm,params:[csrf],response:'DesktopOk'}),
 op(`${D}/apps`,'get','desktopApps','App IDs the user may open (names come from manifests and catalogs)',{...dm,response:'DesktopAppList'}),
 op(`${D}/preferences`,'get','desktopGetPreferences','The user\'s Desktop layout and notes; ETag is the version',{...dm,response:'DesktopPreferences'}),
 op(`${D}/preferences`,'put','desktopPutPreferences','Replace the layout; If-Match must be the ETag last read (412 VERSION_CONFLICT otherwise)',{...dm,params:[csrf,ifMatch],body:'DesktopPreferences',response:'DesktopOk'}),
 op(`${D}/export`,'get','desktopExport','Download the user\'s own Desktop data (layout, pinned apps, notes) as JSON',{...dm,response:'DesktopExport'}),
 op(`${D}/system`,'get','desktopSystem','Process and container metrics of the Desktop service',{...dm,response:'DesktopSystemStatus'}),
 op(`${D}/setup`,'get','desktopSetupStatus','Whether the first-run setup is still open (no account exists yet)',{...dm,security:null,response:'DesktopSetupStatus'}),
 op(`${D}/setup`,'post','desktopSetup','First-run setup: create the System Admin and name the first organization (setup code = the bootstrap secret; 409 SETUP_DONE once any account exists)',{...dm,security:null,body:'DesktopSetupInput',response:'DesktopSetupResult',ok:'201'}),
]
await writeGroup('files-api','files',files,filesOps)
await writeGroup('desktop','desktop',desktop,desktopOps)
const apiPath=`${root}/contracts/api/openapi.yaml`
const api=JSON.parse(await readFile(apiPath,'utf8'))
for(const [path,item] of Object.entries(ops))api.paths[path]={...(api.paths[path]||{}),...item}
for(const [type,schema] of Object.entries({...files,...desktop}))api.components.schemas[type]=schema
await writeFile(apiPath,JSON.stringify(api,null,2)+'\n')
console.log('files ops',filesOps.length,'desktop ops',desktopOps.length)
