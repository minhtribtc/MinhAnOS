import catalogs from '../locales/catalogs.json' with {type:'json'}
export type ActionID='open'|'download'|'copy'|'cut'|'paste'|'rename'|'move'|'delete'|'properties'|'restore'|'mkdir'|'upload'|'refresh'|'grid'|'list'|'select-all'|'favorite'|'purge'|'empty-trash'|'upload-folder'|'share'
export interface FileAction {id:ActionID;label:string;shortcut?:string;disabled?:boolean;danger?:boolean;divider?:boolean}
export interface ActionContext {kind:'empty'|'file'|'folder'|'special'|'trash'|'trash-blank';count:number;system:boolean;canPaste:boolean;canCopy:boolean;readOnly?:boolean;favorite?:boolean;shareable?:boolean}
// A read-only Shared Folder (storage Q2) keeps viewing actions only; the server refuses writes anyway.
const writes=new Set<ActionID>(['mkdir','upload','upload-folder','paste','cut','rename','move','delete','restore','purge','empty-trash'])
// Menus, toolbar, overflow controls and keyboard all dispatch these same action IDs.
export function fileActions(c:ActionContext,t:(key:string)=>string=(key)=>catalogs['vi-VN'][key as keyof typeof catalogs['vi-VN']]||'['+key+']'):FileAction[]{
 const list=actionList(c,t)
 return c.readOnly?list.map(a=>writes.has(a.id)?{...a,disabled:true}:a):list
}
function actionList(c:ActionContext,t:(key:string)=>string):FileAction[]{
 if(c.kind==='empty')return [{id:'mkdir',label:t('files.actions.mkdir')},{id:'upload',label:t('files.actions.upload')},{id:'upload-folder',label:t('files.actions.uploadFolder')},{id:'paste',label:t('files.actions.paste'),shortcut:'Ctrl+V',disabled:!c.canPaste,divider:true},{id:'select-all',label:t('files.actions.selectAll'),shortcut:'Ctrl+A'},{id:'grid',label:t('files.actions.grid'),divider:true},{id:'list',label:t('files.actions.list')},{id:'refresh',label:t('files.actions.refresh')}]
 if(c.kind==='trash-blank')return [{id:'empty-trash',label:t('files.actions.emptyTrash'),danger:true,disabled:!c.count},{id:'refresh',label:t('files.actions.refresh'),divider:true}]
 if(c.kind==='trash')return [{id:'restore',label:t('files.actions.restore')},{id:'properties',label:t('files.actions.properties')},{id:'purge',label:t('files.actions.purge'),danger:true,divider:true}]
 return [{id:'open',label:c.kind==='folder'?t('files.actions.openFolder'):t('files.actions.viewText'),shortcut:'Enter',disabled:c.count!==1||c.kind==='special'},
 {id:'download',label:c.count>1||c.kind==='folder'?t('files.actions.downloadZip'):t('files.actions.download'),disabled:c.kind==='special'},
 {id:'favorite',label:c.favorite?t('files.actions.unfavorite'):t('files.actions.favorite'),disabled:c.count!==1||c.kind==='special'},
 ...(c.shareable?[{id:'share' as const,label:t('files.actions.share'),disabled:c.count!==1||c.kind==='special'}]:[]),
 {id:'copy',label:t('files.actions.copy'),shortcut:'Ctrl+C',disabled:!c.canCopy,divider:true},
 {id:'cut',label:t('files.actions.cut'),shortcut:'Ctrl+X',disabled:c.kind==='special'},
 {id:'paste',label:c.kind==='folder'?t('files.actions.pasteInto'):t('files.actions.paste'),shortcut:'Ctrl+V',disabled:!c.canPaste||c.count!==1||c.kind!=='folder'},
 {id:'rename',label:t('files.actions.rename'),shortcut:'F2',disabled:c.count!==1||c.kind==='special',divider:true},
 {id:'move',label:t('files.actions.move'),disabled:c.count!==1||c.kind==='special'},
 {id:'delete',label:c.system?t('files.actions.remove'):t('files.actions.trash'),shortcut:'Delete',disabled:c.kind==='special',danger:c.system},
 {id:'properties',label:t('files.actions.properties'),divider:true}]
}
