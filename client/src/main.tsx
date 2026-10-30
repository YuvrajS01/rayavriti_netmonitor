import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './fonts.css'
import '@fontsource-variable/instrument-sans'
import '@fontsource-variable/jetbrains-mono'
import App from './App'
import { restoreSession } from './api/auth'
import { reloadOnceForNewBuild } from './utils/lazyWithRetry'
import { initTheme } from './theme/theme'

// public/theme-init.js already set data-theme before first paint; this keeps
// it in sync with the OS while "system" is selected.
initTheme()

// Vite fires this when a modulepreload/CSS dependency of a lazy chunk 404s
// (usually a deploy replaced the hashed assets): reload once to pick up
// the new build instead of failing the navigation.
window.addEventListener('vite:preloadError', (event) => {
  if (reloadOnceForNewBuild()) event.preventDefault()
})

// Restore the session from the HttpOnly refresh cookie before first paint
// of any protected route; ProtectedRoute shows a loader meanwhile.
void restoreSession()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
