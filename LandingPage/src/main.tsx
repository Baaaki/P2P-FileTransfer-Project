import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// Self-hosted so the page makes no third-party requests. Only the prose
// font is bundled: the terminal deliberately uses the visitor's own
// monospace font, which is the one with full box-drawing coverage.
import '@fontsource-variable/inter/wght.css'
import '@fontsource/press-start-2p/latin.css'
import '@fontsource/press-start-2p/latin-ext.css'
import './index.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
