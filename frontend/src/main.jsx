import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './tailwind.css'
import App from './App.jsx'
import { startVisibilityReporting } from './lib/visibilityReporter'

// A backgrounded tab keeps its sockets open, so the server cannot tell "connected"
// from "watching" without being told. Matchmaking needs the difference.
startVisibilityReporting()

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
