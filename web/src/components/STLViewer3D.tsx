import { useEffect, useRef, useState } from 'react'
import * as THREE from 'three'
import { STLLoader } from 'three/examples/jsm/loaders/STLLoader.js'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { Loader2 } from 'lucide-react'

interface STLViewer3DProps {
  url: string
}

export function STLViewer3D({ url }: STLViewer3DProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!containerRef.current) return

    const container = containerRef.current
    const width = container.clientWidth
    const height = 400 // Fixed height for modal

    // Setup Scene
    const scene = new THREE.Scene()
    scene.background = new THREE.Color('#1e293b') // surface-800 to match theme

    // Setup Camera
    const camera = new THREE.PerspectiveCamera(35, width / height, 0.1, 10000)

    // Setup Renderer
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false })
    renderer.setSize(width, height)
    renderer.setPixelRatio(window.devicePixelRatio)
    container.appendChild(renderer.domElement)

    // Setup Controls
    const controls = new OrbitControls(camera, renderer.domElement)
    controls.enableDamping = true
    controls.dampingFactor = 0.05

    // Lighting (Match thumbnail style)
    scene.add(new THREE.HemisphereLight('#ffffff', '#475569', 2.4))
    const keyLight = new THREE.DirectionalLight('#ffffff', 2.8)
    keyLight.position.set(3, 4, 5)
    scene.add(keyLight)
    
    const fillLight = new THREE.DirectionalLight('#93c5fd', 1.1)
    fillLight.position.set(-4, 2, -3)
    scene.add(fillLight)

    let mesh: THREE.Mesh | null = null

    // Load STL
    const loader = new STLLoader()
    loader.load(
      url,
      (geometry) => {
        geometry.computeVertexNormals()
        geometry.computeBoundingBox()
        geometry.computeBoundingSphere()

        const box = geometry.boundingBox
        const sphere = geometry.boundingSphere
        if (!box || !sphere || !Number.isFinite(sphere.radius) || sphere.radius <= 0) {
          setError('Invalid STL geometry')
          setLoading(false)
          return
        }

        // Center the geometry
        const center = new THREE.Vector3()
        box.getCenter(center)
        geometry.translate(-center.x, -center.y, -center.z)
        geometry.computeBoundingSphere()

        const material = new THREE.MeshStandardMaterial({
          color: '#1883FF', // PicoFarm accent
          roughness: 0.62,
          metalness: 0.02,
        })
        
        mesh = new THREE.Mesh(geometry, material)
        
        // Auto-rotate the model slightly
        mesh.rotation.x = -Math.PI / 2
        
        scene.add(mesh)

        // Adjust camera position to fit object
        const radius = geometry.boundingSphere?.radius || sphere.radius
        const distance = Math.max(radius * 3.2, 10)
        camera.position.set(distance * 0.85, distance * 0.72, distance)
        camera.lookAt(0, 0, 0)
        controls.target.set(0, 0, 0)
        controls.update()

        setLoading(false)
      },
      (_xhr) => {
        // Progress could be updated here if needed
      },
      (err) => {
        console.error('Failed to load STL:', err)
        setError('Failed to load STL file')
        setLoading(false)
      }
    )

    // Animation Loop
    let animationFrameId: number
    const animate = () => {
      animationFrameId = requestAnimationFrame(animate)
      controls.update()
      renderer.render(scene, camera)
    }
    animate()

    // Handle Resize
    const handleResize = () => {
      if (!container) return
      camera.aspect = container.clientWidth / height
      camera.updateProjectionMatrix()
      renderer.setSize(container.clientWidth, height)
    }
    window.addEventListener('resize', handleResize)

    // Cleanup
    return () => {
      window.removeEventListener('resize', handleResize)
      cancelAnimationFrame(animationFrameId)
      controls.dispose()
      
      if (mesh) {
        mesh.geometry.dispose()
        ;(mesh.material as THREE.Material).dispose()
      }
      
      renderer.dispose()
      container.removeChild(renderer.domElement)
    }
  }, [url])

  return (
    <div className="relative w-full rounded-lg overflow-hidden border border-surface-700 bg-surface-800" style={{ height: 400 }}>
      {loading && (
        <div className="absolute inset-0 flex items-center justify-center bg-surface-800/80 z-10">
          <div className="flex flex-col items-center text-surface-400">
            <Loader2 className="h-8 w-8 animate-spin text-accent-500 mb-2" />
            <span className="text-sm font-medium">Loading 3D Model...</span>
          </div>
        </div>
      )}
      {error && (
        <div className="absolute inset-0 flex items-center justify-center bg-surface-800 z-10 text-red-400">
          {error}
        </div>
      )}
      <div ref={containerRef} className="w-full h-full cursor-move" />
    </div>
  )
}
