import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig({plugins:[vue()],resolve:{dedupe:['vue','lucide-vue-next']},server:{proxy:{'/api':'http://localhost:8082'}}})
