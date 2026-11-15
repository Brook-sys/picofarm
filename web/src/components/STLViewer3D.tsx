import { useEffect, useRef, useState } from 'react'
import * as THREE from 'three'
import { STLLoader } from 'three/examples/jsm/loaders/STLLoader.js'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { Loader2 } from 'lucide-react'
import { inspectSTLGeometry, readSTLResponse, stlCameraDistance, stlViewPose, type STLView } from './stlViewerUtils'

interface STLViewer3DProps {
  url: string
}

type ModelInfo = { x: number; y: number; z: number; triangles: number }

export function STLViewer3D({ url }: STLViewer3DProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<((view: STLView) => void) | null>(null)
  const [state, setState] = useState<{ url: string; loading: boolean; error: string | null; info: ModelInfo | null }>({ url, loading: true, error: null, info: null })
  const current = state.url === url ? state : { loading: true, error: null, info: null }

  useEffect(() => {
    if (!containerRef.current) return
    const container = containerRef.current
    const abort = new AbortController()
    let disposed = false
    let frame = 0
    let renderer: THREE.WebGLRenderer | undefined
    let controls: OrbitControls | undefined
    let geometry: THREE.BufferGeometry | undefined
    let material: THREE.MeshStandardMaterial | undefined
    let observer: ResizeObserver | undefined
    let radius = 0
    let activeView: STLView = 'reset'
    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#1e293b')
    const camera = new THREE.PerspectiveCamera(35, 1, 0.1, 10000)

    const fail = (err: unknown) => {
      if (disposed) return
      setState({ url, loading: false, error: err instanceof Error ? err.message : 'Could not display the STL. Try another file or a WebGL-capable browser.', info: null })
    }
    const requestRender = () => {
      if (disposed || frame || !renderer) return
      frame = requestAnimationFrame(() => {
        frame = 0
        if (disposed) return
        try {
          controls?.update()
          renderer?.render(scene, camera)
        } catch {
          fail(new Error('3D rendering failed. Try a smaller STL or reload the viewer.'))
        }
      })
    }
    const setView = (view: STLView) => {
      if (!radius || !controls) return
      activeView = view
      const pose = stlViewPose(view)
      const distance = stlCameraDistance(radius, camera.aspect, camera.fov)
      camera.up.copy(pose.up)
      camera.position.copy(pose.direction.multiplyScalar(distance))
      camera.near = Math.max(radius / 1000, 0.000001)
      camera.far = distance + radius * 100
      camera.updateProjectionMatrix()
      controls.target.set(0, 0, 0)
      controls.minDistance = radius * 0.05
      controls.maxDistance = distance * 10
      camera.lookAt(controls.target)
      controls.update()
      requestRender()
    }
    const resize = () => {
      if (disposed || !renderer) return
      const width = Math.max(container.clientWidth, 1)
      const height = Math.max(container.clientHeight, 1)
      camera.aspect = width / height
      camera.updateProjectionMatrix()
      renderer.setSize(width, height)
      if (radius) setView(activeView)
      requestRender()
    }
    const contextLost = (event: Event) => {
      event.preventDefault()
      fail(new Error('The browser lost its 3D context. Close and reopen the viewer.'))
    }

    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false })
      renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
      renderer.domElement.setAttribute('aria-label', 'Interactive STL preview. Drag to rotate, scroll to zoom.')
      renderer.domElement.addEventListener('webglcontextlost', contextLost)
      container.appendChild(renderer.domElement)
      controls = new OrbitControls(camera, renderer.domElement)
      // No idle animation loop or damping: redraw only on interaction/load/resize.
      controls.addEventListener('change', requestRender)
      scene.add(new THREE.HemisphereLight('#ffffff', '#475569', 2.4))
      const keyLight = new THREE.DirectionalLight('#ffffff', 2.8)
      keyLight.position.set(3, 4, 5)
      scene.add(keyLight)
      const fillLight = new THREE.DirectionalLight('#93c5fd', 1.1)
      fillLight.position.set(-4, 2, -3)
      scene.add(fillLight)
      observer = new ResizeObserver(resize)
      observer.observe(container)
      resize()
      viewRef.current = setView

      void (async () => {
        try {
          const response = await fetch(url, { signal: abort.signal })
          const data = await readSTLResponse(response, abort.signal)
          if (disposed) return
          geometry = new STLLoader().parse(data)
          const info = inspectSTLGeometry(geometry)
          geometry.translate(-info.center.x, -info.center.y, -info.center.z)
          material = new THREE.MeshStandardMaterial({ color: '#1883FF', roughness: 0.62, metalness: 0.02 })
          const mesh = new THREE.Mesh(geometry, material)
          mesh.rotation.x = -Math.PI / 2
          scene.add(mesh)
          radius = info.radius
          setView('reset')
          setState({ url, loading: false, error: null, info: { x: info.dimensions.x, y: info.dimensions.y, z: info.dimensions.z, triangles: info.triangles } })
        } catch (err) {
          geometry?.dispose()
          geometry = undefined
          material?.dispose()
          material = undefined
          fail(err)
        }
      })()
    } catch {
      fail(new Error('Could not initialize 3D preview. Check that WebGL is available in your browser.'))
    }

    return () => {
      disposed = true
      abort.abort()
      viewRef.current = null
      observer?.disconnect()
      if (frame) cancelAnimationFrame(frame)
      controls?.removeEventListener('change', requestRender)
      controls?.dispose()
      geometry?.dispose()
      material?.dispose()
      scene.clear()
      if (renderer) {
        renderer.domElement.removeEventListener('webglcontextlost', contextLost)
        renderer.dispose()
        renderer.forceContextLoss()
        renderer.domElement.remove()
      }
    }
  }, [url])

  return (
    <div className="w-full rounded-lg overflow-hidden border border-surface-700 bg-surface-800">
      <div className="flex flex-wrap items-center gap-2 border-b border-surface-700 p-2" role="group" aria-label="STL camera views">
        {([['top', 'Top'], ['front', 'Front'], ['side', 'Side'], ['reset', 'Reset']] as const).map(([view, label]) => (
          <button key={view} type="button" disabled={current.loading || !!current.error} onClick={() => viewRef.current?.(view)} className="rounded border border-surface-600 px-3 py-1 text-sm text-surface-200 hover:bg-surface-700 disabled:opacity-50">{label}</button>
        ))}
      </div>
      <div className="relative" style={{ height: 400 }}>
        {current.loading && (
          <div role="status" className="absolute inset-0 flex items-center justify-center bg-surface-800/80 z-10">
            <div className="flex flex-col items-center text-surface-400">
              <Loader2 className="h-8 w-8 animate-spin text-accent-500 mb-2" />
              <span className="text-sm font-medium">Loading 3D Model...</span>
            </div>
          </div>
        )}
        {current.error && <div role="alert" className="absolute inset-0 flex items-center justify-center bg-surface-800 z-10 text-red-400 p-4 text-center">{current.error}</div>}
        <div ref={containerRef} className="w-full h-full cursor-move" />
      </div>
      <div className="border-t border-surface-700 p-3 text-sm text-surface-300" aria-live="polite">
        {current.info && <p>X: {current.info.x.toLocaleString(undefined, { maximumFractionDigits: 2 })} mm · Y: {current.info.y.toLocaleString(undefined, { maximumFractionDigits: 2 })} mm · Z: {current.info.z.toLocaleString(undefined, { maximumFractionDigits: 2 })} mm · Triangles: {current.info.triangles.toLocaleString()}</p>}
        <p className="text-xs text-surface-400">STL has no unit metadata: millimeters (mm) are assumed. Dimensions use the original STL X/Y/Z axes, before preview rotation. Top looks down Z, front along −Y, side along +X.</p>
      </div>
    </div>
  )
}
