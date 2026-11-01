import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './app/App'
import { BrowserRouter } from 'react-router-dom'
import '@xyflow/react/dist/style.css'
import './styles.css'
import './styles-auth.css'
import './theme-dark.css'
import './landing/landing.css'

document.documentElement.classList.toggle('no-view-transitions', typeof document.startViewTransition !== 'function')

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode><BrowserRouter useTransitions={false}><App /></BrowserRouter></React.StrictMode>,
)
