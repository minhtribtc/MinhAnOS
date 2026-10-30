import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
export default defineConfig({base:'/accounts/',plugins:[vue()],resolve:{dedupe:['vue','lucide-vue-next']},server:{proxy:{'/api':'http://localhost:8081'}}})
