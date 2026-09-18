import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Served from a project subpath by default (GitHub Pages friendly);
  // override with BASE_URL=/ for a root deploy.
  base: process.env.BASE_URL ?? '/',
})
