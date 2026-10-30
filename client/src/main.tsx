import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import './fonts.css'
import '@fontsource-variable/material-symbols-outlined'
import '@fontsource-variable/instrument-sans'
import '@fontsource-variable/plus-jakarta-sans'
import '@fontsource-variable/jetbrains-mono'
import App from './App'
import { restoreSession } from './api/auth'

// Restore the session from the HttpOnly refresh cookie before first paint
// of any protected route; ProtectedRoute shows a loader meanwhile.
void restoreSession()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
