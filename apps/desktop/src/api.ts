import {createApiClient} from '@minhanos/api-client'
export interface User { id: string; username: string; displayName: string; role: 'admin' | 'user' }
export interface AppDefinition { id: string; title: string; subtitle: string; icon: string; color: string; category: string; status: string }
export interface DesktopWindow { id: string; x: number; y: number; width: number; height: number; minimized: boolean; maximized: boolean }
export interface Preferences { theme: 'light' | 'dark'; wallpaper: 'mountain' | 'midnight' | 'sage'; mode: 'focus' | 'windows'; displayName?: string; windows: DesktopWindow[]; notes: string; pinned: string[] }
export interface SystemInfo { version: string; node: string; hostname: string; os: string; architecture: string; goVersion: string; cpus: number; goroutines: number; heapBytes: number; uptimeSeconds: number; diskTotalBytes: number; diskFreeBytes: number; database: string; scope: string; integrations: {id: string; name: string; status: string; description: string}[] }
export interface AuditEntry { id: number; action: string; actor: string; detail: string; createdAt: string }

const client=createApiClient('/api')
export const setCSRF=client.setCSRF
export const api=client.request
export function bytes(value: number) {
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(0)} KB`
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`
  return `${(value / 1024 ** 3).toFixed(1)} GB`
}
