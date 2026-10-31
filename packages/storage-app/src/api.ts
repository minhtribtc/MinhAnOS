// Storage Manager API types (/api/storage/...). Usage is the soft-quota ledger: logical bytes written
// through MinhAnOS; physicalBytes is what ZFS/Btrfs report for a share (null for plain volumes).
import type {StorageHome,StorageShare,StorageQuotaFolder,StorageShareAccess,StorageSubjectPage,StorageVolume} from '@minhanos/shared-types'
export type Home=StorageHome
export type Share=StorageShare
export type Volume=StorageVolume
// Share permissions (Q2): users and groups; deny > write > read; no entry = no access.
export type AccessEntry=StorageShareAccess['entries'][number]
export type Subject=StorageSubjectPage['items'][number]
// Quota folder (Q3): a registered folder of a Shared Folder with its own soft quota.
export type QuotaFolder=StorageQuotaFolder

const GB=1024**3
/** Text in GB to bytes; empty means unlimited (null); undefined means invalid input. */
export function fromGB(value:string):number|null|undefined{
 const v=value.trim().replace(',','.')
 if(!v)return null
 const n=Number(v)
 return Number.isFinite(n)&&n>=0&&n<=1e9?Math.round(n*GB):undefined
}
export function toGB(bytes:number|null):string{return bytes==null?'':String(Math.round(bytes/GB*100)/100)}
