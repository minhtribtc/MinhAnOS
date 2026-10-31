import { Bell, ScrollText, HardDrive, Folder, Settings, NotebookPen, ShieldCheck, AppWindow } from 'lucide-vue-next'
// Icon names used by the backend registry (GET /api/apps); unknown names fall back to a generic app icon.
export const icons: Record<string, typeof Folder> = { files: Folder, settings: Settings, notes: NotebookPen, shield: ShieldCheck, storage: HardDrive, bell: Bell, logs: ScrollText, app: AppWindow }
