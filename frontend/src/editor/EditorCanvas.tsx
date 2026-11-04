import { useEffect, useMemo, useRef, useState } from 'react'
import { ReactFlow, ReactFlowProvider, Background, Controls, useReactFlow, type Connection, type NodeChange, type EdgeChange, type Edge } from '@xyflow/react'
import { Trash2 } from 'lucide-react'
import ArchitectureNode from './ArchitectureNode'
import RequestEdge from './RequestEdge'
import { toFlowEdges, toFlowNodes, type EditorNode } from './graph'
import type { Architecture, ArchitectureEdge, Snapshot } from '../simulation/types'

const nodeTypes = { architecture: ArchitectureNode }
const edgeTypes = { request: RequestEdge }

interface Props {
  architecture: Architecture
  selectedId: string | null
  onSelect: (id: string | null) => void
  onMove: (id: string, position: { x: number; y: number }) => void
  onMoveStart: () => void
  onMoveStop: (id: string, position: { x: number; y: number }) => void
  onConnect: (connection: Connection) => void
  onDeleteNode: (id: string) => void
  onDeleteEdge: (id: string) => void
  onChangeEdgeRoute: (id: string, route: ArchitectureEdge['route']) => void
  functionalRouting: boolean
  snapshot?: Snapshot
  previousSnapshot?: Snapshot
  flowActive: boolean
  flowPaused: boolean
}

export default function EditorCanvas(props: Props) {
  return <ReactFlowProvider><Canvas {...props} /></ReactFlowProvider>
}

function Canvas({ architecture, selectedId, onSelect, onMove, onMoveStart, onMoveStop, onConnect, onDeleteNode, onDeleteEdge, onChangeEdgeRoute, functionalRouting, snapshot, previousSnapshot, flowActive, flowPaused }: Props) {
  const { fitView } = useReactFlow()
  const canvasRef = useRef<HTMLDivElement>(null)
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null)
  const selectedEdge = architecture.edges.find(edge => edge.id === selectedEdgeId)
  const nodeIds = architecture.nodes.map(node => node.id).join(',')
  useEffect(() => {
    if (selectedEdgeId && !selectedEdge) setSelectedEdgeId(null)
  }, [selectedEdgeId, selectedEdge])
  useEffect(() => {
    let frame = 0
    const refit = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => void fitView({ padding: 0.12, maxZoom: 1.1, duration: 0 }))
    }
    const observer = new ResizeObserver(refit)
    if (canvasRef.current) observer.observe(canvasRef.current)
    refit()
    return () => { observer.disconnect(); cancelAnimationFrame(frame) }
  }, [fitView])
  useEffect(() => {
    const frame = requestAnimationFrame(() => void fitView({ padding: 0.12, maxZoom: 1.1, duration: 150 }))
    return () => cancelAnimationFrame(frame)
  }, [fitView, nodeIds])
  const nodes = useMemo(() => toFlowNodes(architecture, snapshot?.nodeMetrics, snapshot?.offeredRps).map(node => ({ ...node, selected: node.id === selectedId })), [architecture, selectedId, snapshot])
  const edges = useMemo(() => toFlowEdges(architecture).map(edge => {
    const total = snapshot?.edgeTraversals?.[edge.id] ?? 0
    const previous = previousSnapshot?.edgeTraversals?.[edge.id] ?? 0
    const recent = Math.max(0, total - previous)
    return { ...edge, type: 'request', selected: edge.id === selectedEdgeId, data: { active: flowActive && (recent > 0 || total > 0), paused: flowPaused, count: Math.min(4, Math.max(1, Math.ceil(Math.log10(Math.max(1, recent + 1))))) } }
  }), [architecture, snapshot, previousSnapshot, flowActive, flowPaused, selectedEdgeId])
  const onNodesChange = (changes: NodeChange<EditorNode>[]) => {
    for (const change of changes) {
      if (change.type === 'position' && change.position) onMove(change.id, change.position)
      if (change.type === 'remove') onDeleteNode(change.id)
    }
  }
  const onEdgesChange = (changes: EdgeChange<Edge>[]) => {
    for (const change of changes) {
      if (change.type === 'select' && change.selected) setSelectedEdgeId(change.id)
      if (change.type === 'remove') { onDeleteEdge(change.id); setSelectedEdgeId(null) }
    }
  }

  return <div ref={canvasRef} className="canvas-wrap" aria-label="Architecture canvas">
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onConnect={onConnect}
      onEdgeClick={(_, edge) => { onSelect(null); setSelectedEdgeId(edge.id) }}
      onNodeClick={(_, node) => { setSelectedEdgeId(null); onSelect(node.id) }}
      onNodeDragStart={onMoveStart}
      onNodeDragStop={(_, node) => onMoveStop(node.id, node.position)}
      onPaneClick={() => { setSelectedEdgeId(null); onSelect(null) }}
      fitView
      fitViewOptions={{ padding: 0.1, maxZoom: 1.15 }}
      minZoom={0.3}
      maxZoom={1.6}
      deleteKeyCode={['Backspace', 'Delete']}
    >
      <Background color="#243342" gap={22} size={1.4} />
      <Controls showInteractive={false} />
    </ReactFlow>
    {selectedEdge && <div className="selected-edge-tools" role="group" aria-label="Selected connection">
      <span>{architecture.nodes.find(node => node.id === selectedEdge.source)?.label ?? selectedEdge.source} → {architecture.nodes.find(node => node.id === selectedEdge.target)?.label ?? selectedEdge.target}</span>
      {functionalRouting && ['api', 'worker'].includes(architecture.nodes.find(node => node.id === selectedEdge.source)?.kind ?? '') && <label>When <select aria-label="Connection operation" value={selectedEdge.route} onChange={event => onChangeEdgeRoute(selectedEdge.id, event.target.value as ArchitectureEdge['route'])}><option value="always">Any request</option><option value="read">Read / redirect</option><option value="write">Write / create</option></select></label>}
      <button type="button" onClick={() => { onDeleteEdge(selectedEdge.id); setSelectedEdgeId(null) }}><Trash2 size={14} />Delete connection</button>
    </div>}
  </div>
}

