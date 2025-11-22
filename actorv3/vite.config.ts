import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  base: '/actor/',
  plugins: [tailwindcss(), vue()],
    build: {
        outDir: '../assets/static/dist',
        manifest: true, // 生成 manifest.json
        emptyOutDir: true
    },
   server: {
        // 如果使用docker-compose开发模式，设置为false
        proxy: {
            '^/api/.*': 'http://localhost:9090',
        }
    },
})
