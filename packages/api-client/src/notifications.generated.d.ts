// Generated from OpenAPI Notifications operations. Do not edit.
import type {NotificationMarkRead,NotificationMarkReadResult,NotificationPage} from '@minhanos/shared-types'
export function createNotificationsClient(base?:string):{setCSRF(value:string):void;notificationsListInbox(args?:{limit?:number;cursor?:string;unread?:string}):Promise<NotificationPage>;notificationsMarkRead(args:{idempotencyKey:string;body:NotificationMarkRead}):Promise<NotificationMarkReadResult>}
