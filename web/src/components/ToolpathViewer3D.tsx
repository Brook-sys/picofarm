import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import * as THREE from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { Loader2 } from 'lucide-react'
import { stlCameraDistance, stlViewPose, type STLView } from './stlViewerUtils'
import { EXTRUSION_CATEGORIES, EXTRUSION_TRIANGLES_PER_SEGMENT, buildExtrusionMeshes, extrusionInstances, type ExtrusionGeometryData, type Visibility } from './toolpath3D'
import type { ToolpathLayer } from '../lib/slicerToolpaths'

const colors = { walls: '#ffca58', infill: '#ed7d31', bridge: '#e879f9', support: '#38bdf8', brim: '#4ade80', other: '#f87171' }
const initialVisibility = Object.fromEntries(EXTRUSION_CATEGORIES.map(category => [category, true])) as Visibility

interface ToolpathBounds {
  minX: number
  minY: number
  minZ: number
  width: number
  height: number
  depth: number
}

interface GeometryResult {
  layers: ToolpathLayer[]
  maxLayer: number
  visible: Visibility
  groups: ExtrusionGeometryData[]
}

export default function ToolpathViewer3D({ layers, bounds, onSwitch2D }: { layers: ToolpathLayer[]; bounds: ToolpathBounds; onSwitch2D: () => void }) {
  const { t } = useTranslation('library')
  const hostRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<(view: STLView) => void>(() => {})
  const [maxLayer, setMaxLayer] = useState(Math.max(0, layers.length - 1))
  const [visible, setVisible] = useState<Visibility>(initialVisibility)
  const [error, setError] = useState('')
  const [geometryResult, setGeometryResult] = useState<GeometryResult | null>(null)
  const sceneRef = useRef<THREE.Group | null>(null)
  const refreshRef = useRef<() => void>(() => {})
  const selected = Math.min(maxLayer, Math.max(0, layers.length - 1))
  const layer = layers[selected]
  const requestedLayer = layer?.index ?? -1
  const description = t('Layer {{number}} · Z {{z}} mm', { number: layer?.index + 1, z: layer?.z.toFixed(3) })
  const geometryData = geometryResult?.groups ?? []
  const building = !error && (geometryResult?.layers !== layers || geometryResult.maxLayer !== requestedLayer || geometryResult.visible !== visible)
  const triangles = geometryData.reduce((sum, data) => sum + data.segments * EXTRUSION_TRIANGLES_PER_SEGMENT, 0)
  const estimated = geometryData.some(data => data.estimatedSegments)

  useEffect(() => {
    const host = hostRef.current
    if (!host) return
    let disposed = false
    let frame = 0
    let activeView: STLView = 'reset'
    let radius = 1
    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#0f172a')
    const camera = new THREE.PerspectiveCamera(35, 1, 0.01, 10000)
    let renderer: THREE.WebGLRenderer
    try { renderer = new THREE.WebGLRenderer({ antialias: true }) } catch { queueMicrotask(() => setError(t('3D preview requires WebGL. Use a WebGL-capable browser.'))); return }
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 1.5))
    renderer.domElement.setAttribute('aria-label', t('3D sliced toolpath preview'))
    host.appendChild(renderer.domElement)
    const controls = new OrbitControls(camera, renderer.domElement)
    const group = new THREE.Group()
    // G-code Z-up becomes world Y-up. Translation is applied after rotation.
    group.rotation.x = -Math.PI / 2
    group.position.set(-(bounds.minX + bounds.width / 2), -(bounds.minZ + bounds.depth / 2), bounds.minY + bounds.height / 2)
    scene.add(group)
    sceneRef.current = group
    scene.add(new THREE.HemisphereLight('#ffffff', '#334155', 2.7))
    const light = new THREE.DirectionalLight('#ffffff', 2.3)
    light.position.set(100, 180, 80)
    scene.add(light)
    radius = Math.max(Math.hypot(bounds.width, bounds.height, bounds.depth) / 2, 0.001)
    const requestRender = () => {
      if (disposed || frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        if (disposed) return
        try { controls.update(); renderer.render(scene, camera) } catch { setError(t('3D rendering failed. Use the 2D view or reload the preview.')) }
      })
    }
    const setView = (view: STLView) => {
      activeView = view
      const pose = stlViewPose(view)
      const distance = stlCameraDistance(radius, camera.aspect, camera.fov)
      camera.up.copy(pose.up); camera.position.copy(pose.direction.multiplyScalar(distance))
      camera.near = Math.max(radius / 1000, 0.000001)
      camera.far = distance + radius * 100
      camera.updateProjectionMatrix()
      controls.target.set(0, 0, 0); controls.minDistance = radius / 100; controls.maxDistance = distance * 10
      camera.lookAt(controls.target); controls.update(); requestRender()
    }
    const resize = () => {
      camera.aspect = Math.max(host.clientWidth, 1) / Math.max(host.clientHeight, 1)
      camera.updateProjectionMatrix(); renderer.setSize(host.clientWidth, host.clientHeight); setView(activeView)
    }
    const observer = new ResizeObserver(resize)
    observer.observe(host); controls.addEventListener('change', requestRender)
    viewRef.current = setView; refreshRef.current = requestRender; resize()
    const lost = (event: Event) => { event.preventDefault(); setError(t('The browser lost its 3D context. Close and reopen the preview.')) }
    renderer.domElement.addEventListener('webglcontextlost', lost)
    return () => {
      disposed = true; if (frame) cancelAnimationFrame(frame); observer.disconnect(); controls.dispose()
      group.traverse(object => {
        if (object instanceof THREE.Mesh) {
          object.geometry.dispose(); (object.material as THREE.Material).dispose()
          if (object instanceof THREE.InstancedMesh) object.dispose()
        }
      })
      scene.clear(); sceneRef.current = null; renderer.domElement.removeEventListener('webglcontextlost', lost)
      renderer.dispose(); renderer.forceContextLoss(); renderer.domElement.remove()
    }
  }, [bounds, t])

  // Yield once so the loading state paints before CPU-heavy matrix generation.
  // A changed slider/filter cancels stale scheduled work; no partial prefix is built.
  useEffect(() => {
    if (error) return
    let active = true
    const timer = window.setTimeout(() => {
      if (!active || !sceneRef.current) return
      try {
        const groups = buildExtrusionMeshes(layers, requestedLayer, visible)
        if (active) setGeometryResult({ layers, maxLayer: requestedLayer, visible, groups })
      } catch {
        if (active) setError(t('The 3D toolpaths could not be built within this browser’s resources. Use the 2D view.'))
      }
    }, 0)
    return () => { active = false; window.clearTimeout(timer) }
  }, [error, layers, requestedLayer, t, visible])

  useEffect(() => {
    const group = sceneRef.current
    if (!group || !geometryResult) return
    for (const child of [...group.children]) {
      if (child instanceof THREE.Mesh) {
        child.geometry.dispose(); (child.material as THREE.Material).dispose(); group.remove(child)
        if (child instanceof THREE.InstancedMesh) child.dispose()
      }
    }
    let segments = 0
    try {
      for (const data of geometryResult.groups) {
        const mesh = extrusionInstances(data, new THREE.MeshStandardMaterial({ color: colors[data.category], roughness: 0.75 }))
        mesh.name = data.category; mesh.userData.segments = data.segments; group.add(mesh); segments += data.segments
      }
      const canvas = hostRef.current?.querySelector('canvas')
      if (canvas) canvas.dataset.renderedSegments = String(segments)
      refreshRef.current()
    } catch {
      queueMicrotask(() => setError(t('3D rendering failed. Use the 2D view or reload the preview.')))
    }
  }, [geometryResult, t])

  return <section aria-label={t('3D sliced toolpath preview')} className="min-w-0 space-y-3">
    <div className="flex flex-wrap gap-2" role="group" aria-label={t('3D camera views')}>
      {(['top', 'front', 'side', 'reset'] as const).map(view => <button type="button" key={view} disabled={!!error} onClick={() => viewRef.current(view)} className="rounded border border-slate-600 px-3 py-1 text-xs text-slate-200 hover:bg-slate-700 disabled:opacity-50">{t(view === 'reset' ? 'Reset' : view === 'top' ? 'Top' : view === 'front' ? 'Front' : 'Side')}</button>)}
    </div>
    <div className="relative h-[420px] overflow-hidden rounded border border-slate-700 bg-slate-950">
      <div ref={hostRef} className="h-full w-full cursor-move" role="img" aria-label={t('Sliced 3D model. Drag to rotate and scroll to zoom.')} />
      {building && <div role="status" className="absolute inset-0 flex items-center justify-center gap-2 bg-slate-900/80 text-sm text-slate-300"><Loader2 className="size-5 animate-spin" />{t('Building 3D toolpaths…')}</div>}
      {error && <div role="alert" className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-slate-900 p-5 text-center text-sm text-red-300"><span>{error}</span><button type="button" onClick={onSwitch2D} className="rounded border border-slate-500 px-3 py-1 text-slate-100">{t('Use 2D view')}</button></div>}
    </div>
    <div>
      <label className="text-xs text-slate-300" htmlFor="toolpath-3d-layer">{t('Show layers through')}</label>
      <input id="toolpath-3d-layer" type="range" min={0} max={layers.length - 1} value={selected} disabled={!!error || layers.length < 2} onChange={event => setMaxLayer(Number(event.target.value))} aria-valuetext={description} className="block w-full accent-sky-400" />
      <output htmlFor="toolpath-3d-layer" className="text-xs tabular-nums text-slate-300">{description}</output>
    </div>
    <details open className="text-xs text-slate-300">
      <summary className="cursor-pointer py-1">{t('Extrusion visibility')}</summary>
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-2">{EXTRUSION_CATEGORIES.map(category => <label key={category} className="flex cursor-pointer items-center gap-2"><input type="checkbox" checked={visible[category]} disabled={!!error} onChange={event => setVisible(previous => ({ ...previous, [category]: event.target.checked }))} className="accent-sky-400" /><span className="size-3 rounded-sm" style={{ backgroundColor: colors[category] }} />{{ walls: t('Walls'), infill: t('Infill'), bridge: t('Bridges'), support: t('Support & interface'), brim: t('Brim'), other: t('Other extrusion') }[category]}</label>)}</div>
    </details>
    <p className="text-xs text-slate-400">{t('Complete source toolpaths · {{triangles}} rendered triangles', { triangles: triangles.toLocaleString() })}</p>
    {estimated && <p role="status" className="text-xs text-amber-300">{t('Some extrusion dimensions were unavailable; nominal width/height estimates are shown.')}</p>}
    <p className="text-xs text-slate-400">{t('Nominal sliced paths, not a physical simulation or collision check. Printer macros and firmware-only motion may not be represented.')}</p>
  </section>
}
