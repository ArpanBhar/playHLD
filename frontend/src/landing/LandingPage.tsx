import { useEffect, useRef, useState, type MouseEvent } from 'react'
import { ArrowRight, Database, Laptop, Layers3, Pause, Play, Server } from 'lucide-react'

const steps = [
  { title: 'Connect your idea', text: 'Start with a client and a server. Add the components your design needs.' },
  { title: 'Turn up the traffic', text: 'Watch requests move through your system. See load, cache misses, and bottlenecks as they happen.' },
  { title: 'Meet the constraints', text: 'Balance resources, throughput, and P95/P99 latency. Improve your design until every target passes.' },
]

export default function LandingPage({ onNavigate }: { onNavigate: (destination: string) => void }) {
  const rootRef = useRef<HTMLElement>(null)
  const [paused, setPaused] = useState(false)
  const [visible, setVisible] = useState(() => document.visibilityState === 'visible')

  useEffect(() => {
    const observer = new IntersectionObserver(entries => {
      for (const entry of entries) entry.target.classList.toggle('is-in-view', entry.isIntersecting)
    }, { threshold: 0.1 })
    rootRef.current?.querySelectorAll('[data-ambient]').forEach(element => observer.observe(element))
    const onVisibility = () => setVisible(document.visibilityState === 'visible')
    document.addEventListener('visibilitychange', onVisibility)
    return () => { observer.disconnect(); document.removeEventListener('visibilitychange', onVisibility) }
  }, [])

  const follow = (destination: string) => (event: MouseEvent<HTMLAnchorElement>) => {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    event.preventDefault()
    onNavigate(destination)
  }

  return <main ref={rootRef} className="landing-page" data-motion={paused || !visible ? 'paused' : 'playing'}>
    <div className="landing-container">
      <section className="landing-hero" aria-labelledby="landing-title">
        <div className="landing-hero-copy">
          <h1 id="landing-title">Build systems.<br /><span>Break limits.</span></h1>
          <p className="landing-intro">Design architectures, send traffic through them, and turn bottlenecks into better decisions.</p>
          <div className="landing-actions">
            <a className="landing-button landing-button-primary" href="/problems" onClick={follow('/problems')}>Explore problems <ArrowRight size={19} /></a>
            <a className="landing-button landing-button-secondary" href="/dev" onClick={follow('/dev')}>Open playground</a>
          </div>
          <p className="landing-start-note">Start with a client and a server. The rest is up to you.</p>
        </div>
        <div className="landing-hero-visual" data-ambient>
          <img src="/images/system-network-1280.webp" srcSet="/images/system-network-768.webp 768w, /images/system-network-1280.webp 1280w" sizes="(max-width: 700px) 100vw, 75vw" width="1280" height="853" fetchPriority="high" alt="An isometric network of servers and a glass database connected by glowing mint pathways" />
        </div>
        <button className="landing-motion-toggle" type="button" aria-label={paused ? 'Play decorative animations' : 'Pause decorative animations'} title={paused ? 'Play animations' : 'Pause animations'} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={14} /> : <Pause size={14} />}</button>
      </section>

      <div className="landing-topics" aria-label="Explore system design concepts"><span>Load balancing</span><span>Caching</span><span>Message queues</span><span>Databases</span></div>

      <section className="landing-workflow" aria-labelledby="workflow-title">
        <div className="landing-workflow-copy">
          <h2 id="workflow-title">Every decision<br /><span>has a consequence.</span></h2>
          <p>Practice the tradeoffs that make a system work. No prescribed architecture. Just clear goals and a finite resource budget.</p>
          <div className="landing-diagram" role="img" aria-label="A client connects to an API, a cache, and a database" data-ambient>
            <svg className="landing-diagram-links" viewBox="0 0 520 112" aria-hidden="true">
              <path className="landing-diagram-line" d="M61 35H151M211 35H301M361 35H451" />
              <path className="landing-diagram-arrows" d="m145 30 6 5-6 5m150-10 6 5-6 5m150-10 6 5-6 5" />
              <path className="landing-flow-path" d="M61 35H151M211 35H301M361 35H451" />
            </svg>
            <div className="landing-diagram-nodes" aria-hidden="true"><div><Laptop /><span>Client</span></div><div><Server /><span>API</span></div><div><Layers3 /><span>Cache</span></div><div><Database /><span>Database</span></div></div>
          </div>
        </div>
        <ol className="landing-steps">{steps.map((step, index) => <li key={step.title}><span className="landing-step-number">0{index + 1}</span><div><h3>{step.title}</h3><p>{step.text}</p></div></li>)}</ol>
      </section>

      <section className="landing-final-cta" aria-labelledby="landing-cta-title"><div><h2 id="landing-cta-title">Your next architecture starts here.</h2><p>Pick a challenge. Build your own solution.</p></div><a className="landing-button landing-button-primary" href="/problems" onClick={follow('/problems')}>Explore problems <ArrowRight size={19} /></a></section>
      <footer className="landing-footer"><div className="landing-footer-brand"><img src="/favicon.svg" width="30" height="30" alt="SystemForge" /><span>System design, made tangible.</span></div><nav aria-label="Footer"><a href="/problems" onClick={follow('/problems')}>Problems</a><a href="/dev" onClick={follow('/dev')}>Dev</a><a href="/contest" onClick={follow('/contest')}>Contest</a></nav></footer>
    </div>
  </main>
}
