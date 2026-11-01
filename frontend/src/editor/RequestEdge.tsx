import { memo, useEffect, useRef } from 'react'
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath, type EdgeProps } from '@xyflow/react'

function RequestEdge({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, label, data, selected }: EdgeProps) {
  const particlesRef = useRef<SVGGElement>(null)
  const [path, labelX, labelY] = getSmoothStepPath({ sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition })
  const active = data?.active === true
  const paused = data?.paused === true
  const count = typeof data?.count === 'number' ? Math.max(1, Math.min(4, data.count)) : 1
  useEffect(() => {
    const svg = particlesRef.current?.ownerSVGElement
    if (!svg) return
    if (paused) svg.pauseAnimations()
    else svg.unpauseAnimations()
  }, [paused, active])
  return <>
    <BaseEdge id={id} path={path} style={{ stroke: selected ? '#1767ed' : '#8998ad', strokeWidth: selected ? 2.5 : 2 }} />
    {active && <g ref={particlesRef} className="request-particles" aria-hidden="true">
      {Array.from({ length: count }, (_, index) => <circle key={index} r="3.3" fill="#1767ed" stroke="#fff" strokeWidth="1.2">
        <animateMotion dur={`${2.1 - count * 0.16}s`} begin={`${-index * 0.55}s`} repeatCount="indefinite" path={path} />
      </circle>)}
    </g>}
    {label && <EdgeLabelRenderer><div className="request-edge-label" style={{ transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)` }}>{label}</div></EdgeLabelRenderer>}
  </>
}

export default memo(RequestEdge)
