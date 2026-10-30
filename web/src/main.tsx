import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { App } from './App'
import { LiveProvider } from './live'
import { ToastProvider } from './components/ui'
import './styles.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ToastProvider>
      <LiveProvider>
        <App />
      </LiveProvider>
    </ToastProvider>
  </StrictMode>,
)
