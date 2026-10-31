import {defineSettingsProvider} from '@minhanos/app-sdk'
import manifest from '../../../contracts/manifests/files.app.json'
import ActivityPanel from './ActivityPanel.vue'

// Files configuration shown in central Settings: the activity log level (ADR 0012).
export const settingsProvider=defineSettingsProvider({manifest,panels:{activity:ActivityPanel}})
