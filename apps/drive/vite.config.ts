import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig({plugins:[vue()],resolve:{alias:{'@minhanos/files-app':decodeURIComponent(new URL('../../packages/files-app/src/index.ts',import.meta.url).pathname).replace(/^\/([A-Za-z]:)/, '$1')},dedupe:['vue','lucide-vue-next']},server:{port:5174,proxy:{'/api':'http://localhost:8082'}}})
