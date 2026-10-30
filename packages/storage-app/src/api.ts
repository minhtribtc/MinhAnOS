// Storage Manager API types (/api/storage/...). Usage is the soft-quota ledger: logical bytes written
// through MinhAnOS; physicalBytes is what ZFS/Btrfs report for a share (null for plain volumes).
import type {StorageHome,StorageShare,StorageVolume} from '@minhanos/shared-types'
export type Home=StorageHome
export type Share=StorageShare
export type Volume=StorageVolume

const GB=1024**3
/** Text in GB to bytes; empty means unlimited (null); undefined means invalid input. */
export function fromGB(value:string):number|null|undefined{
 const v=value.trim().replace(',','.')
 if(!v)return null
 const n=Number(v)
 return Number.isFinite(n)&&n>=0&&n<=1e9?Math.round(n*GB):undefined
}
export function toGB(bytes:number|null):string{return bytes==null?'':String(Math.round(bytes/GB*100)/100)}
