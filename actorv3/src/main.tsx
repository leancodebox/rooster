import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import '@fortawesome/fontawesome-free/css/all.css'
import './assets/tailwind.css'
import './style.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
