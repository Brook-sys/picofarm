import * as THREE from 'three'
import { STLLoader } from 'three/examples/jsm/loaders/STLLoader.js'

const THUMBNAIL_SIZE = 512
const RENDER_TIMEOUT_MS = 8000

export async function renderSTLThumbnailWithTimeout(file: File): Promise<Blob | null> {
  return Promise.race([
    renderSTLThumbnail(file),
    new Promise<null>(resolve => window.setTimeout(() => resolve(null), RENDER_TIMEOUT_MS)),
  ]).catch(() => null)
}

async function renderSTLThumbnail(file: File): Promise<Blob | null> {
  const buffer = await file.arrayBuffer()
  const loader = new STLLoader()
  const geometry = loader.parse(buffer)
  geometry.computeVertexNormals()
  geometry.computeBoundingBox()
  geometry.computeBoundingSphere()

  const box = geometry.boundingBox
  const sphere = geometry.boundingSphere
  if (!box || !sphere || !Number.isFinite(sphere.radius) || sphere.radius <= 0) return null

  const center = new THREE.Vector3()
  box.getCenter(center)
  geometry.translate(-center.x, -center.y, -center.z)
  geometry.computeBoundingSphere()

  const scene = new THREE.Scene()
  scene.background = new THREE.Color('#1e293b')

  const material = new THREE.MeshStandardMaterial({
    color: '#1883FF',
    roughness: 0.62,
    metalness: 0.02,
  })
  const mesh = new THREE.Mesh(geometry, material)
  scene.add(mesh)

  scene.add(new THREE.HemisphereLight('#ffffff', '#475569', 2.4))
  const key = new THREE.DirectionalLight('#ffffff', 2.8)
  key.position.set(3, 4, 5)
  scene.add(key)
  const fill = new THREE.DirectionalLight('#93c5fd', 1.1)
  fill.position.set(-4, 2, -3)
  scene.add(fill)

  const radius = geometry.boundingSphere?.radius || sphere.radius
  const camera = new THREE.PerspectiveCamera(35, 1, 0.1, Math.max(10000, radius * 20))
  const distance = Math.max(radius * 3.2, 10)
  camera.position.set(distance * 0.85, distance * 0.72, distance)
  camera.lookAt(0, 0, 0)

  const canvas = document.createElement('canvas')
  canvas.width = THUMBNAIL_SIZE
  canvas.height = THUMBNAIL_SIZE
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: false, preserveDrawingBuffer: true })
  renderer.setSize(THUMBNAIL_SIZE, THUMBNAIL_SIZE, false)
  renderer.setPixelRatio(1)
  renderer.render(scene, camera)

  const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/png'))
  geometry.dispose()
  material.dispose()
  renderer.dispose()
  return blob
}
