const API_URL = import.meta.env.VITE_API_URL ?? (typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.host}` : 'http://localhost:8084')

// Generic fetch wrapper with error handling.
async function fetchApi<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const url = `${API_URL}/api${path}`

  console.log(`[API] ${options.method || 'GET'} ${url}`)

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  // Merge any existing headers
  if (options.headers) {
    const existingHeaders = options.headers as Record<string, string>
    Object.assign(headers, existingHeaders)
  }

  const response = await fetch(url, {
    ...options,
    headers,
  })

  console.log(`[API] Response status: ${response.status}`)

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error(error.error || `HTTP ${response.status}`)
  }

  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()
  console.log(`[API] Response body:`, text.substring(0, 500))
  
  try {
    return JSON.parse(text) as T
  } catch (e) {
    console.error('[API] JSON parse error:', e, 'Body was:', text)
    throw e
  }
}

// Projects API
export const projectsApi = {
  list: () =>
    fetchApi<import('../types').Project[]>('/projects'),

  get: (id: string) =>
    fetchApi<import('../types').Project>(`/projects/${id}`),

  create: (data: Partial<import('../types').Project>) =>
    fetchApi<import('../types').Project>('/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Project>) =>
    fetchApi<import('../types').Project>(`/projects/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/projects/${id}`, { method: 'DELETE' }),

  // Project pipeline methods
  clearJobs: (id: string) =>
    fetchApi<void>(`/projects/${id}/jobs`, { method: 'DELETE' }),

  listJobs: (id: string) =>
    fetchApi<import('../types').PrintJob[]>(`/projects/${id}/jobs`),

  getJobStats: (id: string) =>
    fetchApi<import('../types').JobStats>(`/projects/${id}/job-stats`),

  getSummary: (id: string) =>
    fetchApi<import('../types').ProjectSummary>(`/projects/${id}/summary`),

  startProduction: (id: string) =>
    fetchApi<import('../types').StartProductionResult>(`/projects/${id}/start-production`, {
      method: 'POST',
    }),

  markReadyToShip: (id: string) =>
    fetchApi<import('../types').Project>(`/projects/${id}/ready-to-ship`, {
      method: 'POST',
    }),

  ship: (id: string, trackingNumber?: string) =>
    fetchApi<import('../types').Project>(`/projects/${id}/ship`, {
      method: 'POST',
      body: JSON.stringify({ tracking_number: trackingNumber }),
    }),

  // Tasks for this project
  listTasks: (id: string) =>
    fetchApi<import('../types').Task[]>(`/projects/${id}/tasks`),
}

// Tasks API (Work Instances)
export const tasksApi = {
  list: (filters?: { project_id?: string; order_id?: string; status?: string }) => {
    const params = new URLSearchParams()
    if (filters?.project_id) params.set('project_id', filters.project_id)
    if (filters?.order_id) params.set('order_id', filters.order_id)
    if (filters?.status) params.set('status', filters.status)
    const query = params.toString()
    return fetchApi<import('../types').Task[]>(`/tasks${query ? `?${query}` : ''}`)
  },

  get: (id: string) =>
    fetchApi<import('../types').Task>(`/tasks/${id}`),

  create: (data: {
    project_id: string
    order_id?: string
    order_item_id?: string
    name: string
    quantity?: number
    notes?: string
    pickup_date?: string
  }) =>
    fetchApi<import('../types').Task>('/tasks', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<{ name: string; quantity: number; notes: string; pickup_date: string | null }>) =>
    fetchApi<import('../types').Task>(`/tasks/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/tasks/${id}`, { method: 'DELETE' }),

  updateStatus: (id: string, status: import('../types').TaskStatus) =>
    fetchApi<import('../types').Task>(`/tasks/${id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    }),

  getProgress: (id: string) =>
    fetchApi<{ progress: number }>(`/tasks/${id}/progress`),

  start: (id: string) =>
    fetchApi<import('../types').Task>(`/tasks/${id}/start`, { method: 'POST' }),

  complete: (id: string) =>
    fetchApi<import('../types').Task>(`/tasks/${id}/complete`, { method: 'POST' }),

  cancel: (id: string) =>
    fetchApi<import('../types').Task>(`/tasks/${id}/cancel`, { method: 'POST' }),

  getChecklist: (id: string) =>
    fetchApi<import('../types').TaskChecklistItem[]>(`/tasks/${id}/checklist`),

  regenerateChecklist: (id: string) =>
    fetchApi<import('../types').TaskChecklistItem[]>(`/tasks/${id}/checklist/regenerate`, {
      method: 'POST',
    }),

  toggleChecklistItem: (taskId: string, itemId: string, completed: boolean) =>
    fetchApi<{ ok: boolean }>(`/tasks/${taskId}/checklist/${itemId}`, {
      method: 'PATCH',
      body: JSON.stringify({ completed }),
    }),

  printFromChecklist: (taskId: string, itemId: string) =>
    fetchApi<import('../types').PrintJob>(`/tasks/${taskId}/checklist/${itemId}/print`, {
      method: 'POST',
    }),
}

// Parts API
export const partsApi = {
  listByProject: (projectId: string) => 
    fetchApi<import('../types').Part[]>(`/projects/${projectId}/parts`),
  
  get: (id: string) => 
    fetchApi<import('../types').Part>(`/parts/${id}`),
  
  create: (projectId: string, data: Partial<import('../types').Part>) => 
    fetchApi<import('../types').Part>(`/projects/${projectId}/parts`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  
  update: (id: string, data: Partial<import('../types').Part>) => 
    fetchApi<import('../types').Part>(`/parts/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  
  delete: (id: string) =>
    fetchApi<void>(`/parts/${id}`, { method: 'DELETE' }),

  createWithFile: async (
    projectId: string,
    data: Partial<import('../types').Part> & { gcode_file_id?: string; stl_file_id?: string },
    file?: File,
    notes?: string
  ) => {
    if (!file) {
      return partsApi.create(projectId, data)
    }

    const formData = new FormData()
    if (data.name) formData.append('name', data.name)
    if (data.description) formData.append('description', data.description)
    if (data.gcode_file_id) formData.append('gcode_file_id', data.gcode_file_id)
    formData.append('quantity', String(data.quantity || 1))
    formData.append('file', file)
    if (notes) formData.append('notes', notes)

    const response = await fetch(`${API_URL}/api/projects/${projectId}/parts`, {
      method: 'POST',
      body: formData,
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Upload failed' }))
      throw new Error(error.error)
    }

    return response.json()
  },
}

// Project Supplies API
export const suppliesApi = {
  listByProject: (projectId: string) =>
    fetchApi<import('../types').ProjectSupply[]>(`/projects/${projectId}/supplies`),

  create: (projectId: string, data: { name: string; unit_cost_cents: number; quantity: number; notes?: string; material_id?: string }) =>
    fetchApi<import('../types').ProjectSupply>(`/projects/${projectId}/supplies`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/supplies/${id}`, { method: 'DELETE' }),
}

// Designs API
export const designsApi = {
  listByPart: (partId: string) =>
    fetchApi<import('../types').Design[]>(`/parts/${partId}/designs`),

  get: (id: string) =>
    fetchApi<import('../types').Design>(`/designs/${id}`),

  linkGCode: (partId: string, gcodeFileId: string, notes?: string) =>
    fetchApi<import('../types').Design>(`/parts/${partId}/designs`, {
      method: 'POST',
      body: JSON.stringify({ gcode_file_id: gcodeFileId, notes: notes || '' }),
    }),

  linkRootFile: (partId: string, file: { type: 'stl' | 'gcode'; id: string }, notes?: string) =>
    fetchApi<import('../types').Design>(`/parts/${partId}/designs`, {
      method: 'POST',
      body: JSON.stringify({ gcode_file_id: file.type === 'gcode' ? file.id : undefined, stl_file_id: file.type === 'stl' ? file.id : undefined, notes: notes || '' }),
    }),

  upload: async (partId: string, file: File, notes?: string) => {
    const formData = new FormData()
    formData.append('file', file)
    if (notes) formData.append('notes', notes)

    const response = await fetch(`${API_URL}/api/parts/${partId}/designs`, {
      method: 'POST',
      body: formData,
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Upload failed' }))
      throw new Error(error.error)
    }

    return response.json() as Promise<import('../types').Design>
  },

  delete: (id: string) => fetchApi<void>(`/designs/${id}`, { method: 'DELETE' }),

  downloadUrl: (id: string) => `${API_URL}/api/designs/${id}/download`,

  listPrintJobs: (designId: string) =>
    fetchApi<import('../types').PrintJob[]>(`/designs/${designId}/print-jobs`),

  openExternal: (id: string, app?: string) =>
    fetchApi<{ status: string }>(`/designs/${id}/open-external`, {
      method: 'POST',
      body: JSON.stringify({ app: app || '' }),
    }),
}

// Discovered printer from network scan
export interface DiscoveredPrinter {
  id: string
  name: string
  host: string
  port: number
  type: import('../types').ConnectionType
  model?: string
  manufacturer?: string
  version?: string
  serial_number?: string
  already_added: boolean
}

// Printers API
export const slicerApi = {
  getConfig: () => fetchApi<{ connection_url: string }>('/slicer/config'),
  setConfig: (connectionUrl: string) => fetchApi<{ connection_url: string }>('/slicer/config', { method: 'PUT', body: JSON.stringify({ connection_url: connectionUrl }) }),
  health: () => fetchApi<Record<string, unknown>>('/slicer/health'),
  status: () => fetchApi<Record<string, unknown>>('/slicer/status'),
  profiles: (category: 'printers' | 'presets' | 'filaments') => fetchApi<Array<Record<string, unknown>>>(`/slicer/profiles/${category}`),
  profileJSON: (category: string, name: string) => fetchApi<Record<string, unknown>>(`/slicer/profiles/${category}/${encodeURIComponent(name)}`),
  importProfile: (data: { category: string; name: string; url: string; overwrite?: boolean }) => fetchApi<Record<string, unknown>>('/slicer/profiles/import-url', { method: 'POST', body: JSON.stringify(data) }),
  uploadProfileJSON: (data: { category: string; name: string; json: string }) => fetchApi<Record<string, unknown>>('/slicer/profiles/upload-json', { method: 'POST', body: JSON.stringify(data) }),
  updateProfile: (category: string, name: string) => fetchApi<Record<string, unknown>>(`/slicer/profiles/${category}/${encodeURIComponent(name)}/update-from-source`, { method: 'POST' }),
  resolveProfiles: (data: Record<string, unknown>) => fetchApi<Record<string, unknown>>('/slicer/resolve-profiles', { method: 'POST', body: JSON.stringify(data) }),
  sliceSTL: (data: Record<string, unknown>) => fetchApi<Record<string, unknown>>('/slicer/slice-stl', { method: 'POST', body: JSON.stringify(data) }),
}

export const notificationsApi = {
  list: () => fetchApi<import('../types').NotificationChannel[]>('/notifications'),
  create: (data: Partial<import('../types').NotificationChannel>) =>
    fetchApi<import('../types').NotificationChannel>('/notifications', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  update: (id: string, data: Partial<import('../types').NotificationChannel>) =>
    fetchApi<import('../types').NotificationChannel>(`/notifications/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  delete: (id: string) => fetchApi<void>(`/notifications/${id}`, { method: 'DELETE' }),
  test: (id: string) => fetchApi<void>(`/notifications/${id}/test`, { method: 'POST' }),
  deliveries: (channelId?: string) =>
    fetchApi<import('../types').NotificationDelivery[]>(`/notifications/deliveries${channelId ? `?channel_id=${channelId}` : ''}`),
  templates: (channelId?: string) =>
    fetchApi<import('../types').NotificationTemplate[]>(`/notifications/templates${channelId ? `?channel_id=${channelId}` : ''}`),
  saveTemplate: (data: import('../types').NotificationTemplate) =>
    fetchApi<import('../types').NotificationTemplate>('/notifications/templates', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteTemplate: (id: string) => fetchApi<void>(`/notifications/templates/${id}`, { method: 'DELETE' }),
  previewTemplate: (data: import('../types').NotificationTemplate) =>
    fetchApi<import('../types').NotificationPreview>('/notifications/templates/preview', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
}

export const printersApi = {
  list: () => 
    fetchApi<import('../types').Printer[]>('/printers'),
  
  get: (id: string) => 
    fetchApi<import('../types').Printer>(`/printers/${id}`),
  
  create: (data: Partial<import('../types').Printer>) => 
    fetchApi<import('../types').Printer>('/printers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  
  update: (id: string, data: Partial<import('../types').Printer>) => 
    fetchApi<import('../types').Printer>(`/printers/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  
  delete: (id: string) => 
    fetchApi<void>(`/printers/${id}`, { method: 'DELETE' }),

  setMaintenanceMode: (id: string, maintenanceMode: boolean) =>
    fetchApi<import('../types').Printer>(`/printers/${id}/maintenance`, {
      method: 'POST',
      body: JSON.stringify({ maintenance_mode: maintenanceMode }),
    }),

  getDefault: () => fetchApi<import('../types').Printer>('/printers/default'),

  setDefault: (id: string) => fetchApi<void>(`/printers/${id}/default`, { method: 'POST' }),

  emergencyStop: () =>
    fetchApi<void>('/printers/emergency-stop', { method: 'POST' }),

  runMacro: (id: string, macro: string) =>
    fetchApi<void>(`/printers/${id}/macro`, {
      method: 'POST',
      body: JSON.stringify({ macro }),
    }),

  reconnect: (id: string) =>
    fetchApi<void>(`/printers/${id}/reconnect`, { method: 'POST' }),

  listMacros: () => fetchApi<import('../types').PrinterMacro[]>('/printers/macros'),
  createMacro: (title: string, command: string) =>
    fetchApi<import('../types').PrinterMacro>('/printers/macros', {
      method: 'POST',
      body: JSON.stringify({ title, command }),
    }),
  updateMacro: (id: number, title: string, command: string) =>
    fetchApi<import('../types').PrinterMacro>(`/printers/macros/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ title, command }),
    }),
  deleteMacro: (id: number) => fetchApi<void>(`/printers/macros/${id}`, { method: 'DELETE' }),

  setPrintSpeed: (id: string, level: number) =>
    fetchApi<void>(`/printers/${id}/speed`, {
      method: 'POST',
      body: JSON.stringify({ level }),
    }),

  setFeedRate: (id: string, percent: number) =>
    fetchApi<void>(`/printers/${id}/speed`, {
      method: 'POST',
      body: JSON.stringify({ percent }),
    }),

  getCapabilities: (id: string) =>
    fetchApi<import('../types').PrinterCapabilities>(`/printers/${id}/capabilities`),

  setFanSpeed: (id: string, fan: string, speed: number) =>
    fetchApi<void>(`/printers/${id}/fan`, {
      method: 'POST',
      body: JSON.stringify({ fan, speed }),
    }),

  setLEDMode: (id: string, mode: string) =>
    fetchApi<void>(`/printers/${id}/led`, {
      method: 'POST',
      body: JSON.stringify({ mode }),
    }),

  skipObject: (id: string, objectId: string) =>
    fetchApi<void>(`/printers/${id}/skip-object`, {
      method: 'POST',
      body: JSON.stringify({ object_id: objectId }),
    }),

  jog: (id: string, axis: string, distance: number) =>
    fetchApi<void>(`/printers/${id}/jog`, {
      method: 'POST',
      body: JSON.stringify({ axis, distance }),
    }),

  setTemperature: (id: string, heater: string, targetTemp: number) =>
    fetchApi<void>(`/printers/${id}/temperature`, {
      method: 'POST',
      body: JSON.stringify({ heater, target_temp: targetTemp }),
    }),

  plateCleared: (id: string) =>
    fetchApi<void>(`/printers/${id}/plate-cleared`, { method: 'POST' }),

  amsLoad: (id: string, amsId: string, slotId: string) =>
    fetchApi<void>(`/printers/${id}/ams/load`, {
      method: 'POST',
      body: JSON.stringify({ ams_id: amsId, slot_id: slotId }),
    }),

  amsUnload: (id: string) =>
    fetchApi<void>(`/printers/${id}/ams/unload`, { method: 'POST' }),

  amsRefresh: (id: string) =>
    fetchApi<void>(`/printers/${id}/ams/refresh`, { method: 'POST' }),

  setAMSFilamentBackup: (id: string, enabled: boolean) =>
    fetchApi<void>(`/printers/${id}/ams/backup`, {
      method: 'POST',
      body: JSON.stringify({ enabled }),
    }),
  
  getState: (id: string) =>
    fetchApi<import('../types').PrinterState>(`/printers/${id}/state`),

  getAllStates: () =>
    fetchApi<Record<string, import('../types').PrinterState>>('/printers/states'),

  getJobs: (id: string) =>
    fetchApi<import('../types').PrintJob[]>(`/printers/${id}/jobs`),

  clearJobs: (id: string) =>
    fetchApi<void>(`/printers/${id}/jobs`, { method: 'DELETE' }),

  getStats: (id: string) =>
    fetchApi<import('../types').JobStats>(`/printers/${id}/stats`),

  getAnalytics: (id: string) =>
    fetchApi<import('../types').PrinterAnalytics>(`/printers/${id}/analytics`),

  discover: async () => {
    console.log('Starting printer discovery...')
    try {
      const result = await fetchApi<DiscoveredPrinter[]>('/printers/discover', { method: 'POST' })
      console.log('Discovery result:', result)
      return result
    } catch (err) {
      console.error('Discovery error:', err)
      throw err
    }
  },
}

// Cameras API
export const camerasApi = {
  list: () => fetchApi<import('../types').Camera[]>('/cameras'),
  create: (data: Partial<import('../types').Camera>) =>
    fetchApi<import('../types').Camera>('/cameras', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) => fetchApi<void>(`/cameras/${id}`, { method: 'DELETE' }),
}

// Timelapses API
export const timelapsesApi = {
  list: (printerId?: string) =>
    fetchApi<import('../types').Timelapse[]>(`/timelapses${printerId ? `?printer_id=${printerId}` : ''}`),
  create: (data: Partial<import('../types').Timelapse>) =>
    fetchApi<import('../types').Timelapse>('/timelapses', { method: 'POST', body: JSON.stringify(data) }),
}

// Print Archives API
export const archivesApi = {
  list: (printerId?: string, status?: string) =>
    fetchApi<import('../types').PrintArchive[]>(`/archives${printerId ? `?printer_id=${printerId}` : ''}${status ? `${printerId ? '&' : '?'}status=${status}` : ''}`),
  create: (data: Partial<import('../types').PrintArchive>) =>
    fetchApi<import('../types').PrintArchive>('/archives', { method: 'POST', body: JSON.stringify(data) }),
  compare: (a: string, b: string) =>
    fetchApi<unknown>(`/archives/compare?a=${a}&b=${b}`),
  exportCSV: () => `/archives/log/export`,
}

// Materials API
export const materialsApi = {
  list: () =>
    fetchApi<import('../types').Material[]>('/materials'),

  listByType: (type: string) =>
    fetchApi<import('../types').Material[]>(`/materials?type=${encodeURIComponent(type)}`),

  get: (id: string) =>
    fetchApi<import('../types').Material>(`/materials/${id}`),

  create: (data: Partial<import('../types').Material>) =>
    fetchApi<import('../types').Material>('/materials', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Material>) =>
    fetchApi<import('../types').Material>(`/materials/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/materials/${id}`, { method: 'DELETE' }),
}

// Spools API
export const enqueueProjectParts = async (projectId: string, onProgress?: (msg: string) => void) => {
  const parts = await partsApi.listByProject(projectId)
  if (!parts || parts.length === 0) return { added: 0, missing: 0 }

  const library = await fileLibraryApi.get()
  const spools = await spoolsApi.list()
  const materials = await materialsApi.list()
  const spoolsWithMaterials = spools.map(s => ({ ...s, material: materials.find(m => m.id === s.material_id) }))
  const availableSpools = spoolsWithMaterials.filter(s => s.status !== 'empty' && s.status !== 'archived' && s.material)

  let addedCount = 0
  let missingGCodeCount = 0

  for (const part of parts) {
    const designs = await designsApi.listByPart(part.id)
    if (!designs || designs.length === 0) continue

    const latestDesign = [...designs].sort((a, b) => b.version - a.version)[0]
    if (!latestDesign) continue

    let gcodeToPrint: import('../types').GCodeLibraryFile | undefined

    if (latestDesign.file_type === 'stl') {
      const stl = (library.stl_files || []).find(file => file.file_id === latestDesign.file_id)
      gcodeToPrint = stl?.gcodes?.find(file => file.default_for_stl) || stl?.gcodes?.[0]
    } else if (latestDesign.file_type === 'gcode') {
      gcodeToPrint = [...(library.root_gcode_files || []), ...(library.stl_files || []).flatMap(file => file.gcodes || [])].find(file => file.file_id === latestDesign.file_id)
    }

    if (!gcodeToPrint) {
      missingGCodeCount++
      continue
    }

    let assigned_spool_id: string | undefined
    let spoolData: (import('../types').MaterialSpool & { material?: import('../types').Material }) | undefined

    if (gcodeToPrint.material_type) {
      const materialType = gcodeToPrint.material_type.toLowerCase()
      const spool = availableSpools.find(s => s.material?.type.toLowerCase() === materialType)
      if (spool) {
        assigned_spool_id = spool.id
        spoolData = spool
      }
    }

    for (let i = 0; i < part.quantity; i++) {
      if (onProgress) onProgress(`Queueing ${part.name} (${i + 1}/${part.quantity})...`)
      try {
        await gcodeLibraryApi.addToQueue(gcodeToPrint.id, spoolData?.material ? { 
          assigned_spool_id, 
          project_id: part.project_id, 
          material_type: spoolData.material.type, 
          material_color: spoolData.material.color_hex || spoolData.material.color, 
          filament_name: spoolData.material.name, 
          source_type: 'project' 
        } : { 
          project_id: part.project_id, 
          source_type: 'project' 
        })
        addedCount++
      } catch (e) {
        console.error(`Failed to queue ${part.name}:`, e)
      }
    }
  }

  return { added: addedCount, missing: missingGCodeCount }
}

export const spoolsApi = {
  list: () => 
    fetchApi<import('../types').MaterialSpool[]>('/spools'),
  
  get: (id: string) => 
    fetchApi<import('../types').MaterialSpool>(`/spools/${id}`),
  
  create: (data: Partial<import('../types').MaterialSpool>) =>
    fetchApi<import('../types').MaterialSpool>('/spools', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').MaterialSpool>) =>
    fetchApi<import('../types').MaterialSpool>(`/spools/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/spools/${id}`, { method: 'DELETE' }),
}

// Print Jobs API
export const printJobsApi = {
  list: (params?: { printer_id?: string; status?: string }) => {
    const searchParams = new URLSearchParams()
    if (params?.printer_id) searchParams.set('printer_id', params.printer_id)
    if (params?.status) searchParams.set('status', params.status)
    const query = searchParams.toString()
    return fetchApi<import('../types').PrintJob[]>(`/print-jobs${query ? `?${query}` : ''}`)
  },

  get: (id: string) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}`),

  create: (data: Partial<import('../types').PrintJob>) =>
    fetchApi<import('../types').PrintJob>('/print-jobs', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').PrintJob>) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) => fetchApi<void>(`/print-jobs/${id}`, { method: 'DELETE' }),

  updatePriority: (id: string, priority: number) =>
    fetchApi<{ status: string }>(`/print-jobs/${id}/priority`, {
      method: 'PATCH',
      body: JSON.stringify({ priority }),
    }),

  start: (id: string) =>
    fetchApi<void>(`/print-jobs/${id}/start`, { method: 'POST' }),

  pause: (id: string) =>
    fetchApi<void>(`/print-jobs/${id}/pause`, { method: 'POST' }),

  resume: (id: string) =>
    fetchApi<void>(`/print-jobs/${id}/resume`, { method: 'POST' }),

  cancel: (id: string) =>
    fetchApi<void>(`/print-jobs/${id}/cancel`, { method: 'POST' }),

  recordOutcome: (id: string, outcome: import('../types').PrintOutcome) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}/outcome`, {
      method: 'POST',
      body: JSON.stringify(outcome),
    }),

  // Job history methods
  getWithEvents: (id: string) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}/with-events`),

  getEvents: (id: string) =>
    fetchApi<import('../types').JobEvent[]>(`/print-jobs/${id}/events`),

  getRetryChain: (id: string) =>
    fetchApi<import('../types').PrintJob[]>(`/print-jobs/${id}/retry-chain`),

  retry: (id: string, request?: import('../types').RetryJobRequest) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}/retry`, {
      method: 'POST',
      body: JSON.stringify(request || {}),
    }),

  recordFailure: (id: string, request: import('../types').RecordFailureRequest) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}/failure`, {
      method: 'POST',
      body: JSON.stringify(request),
    }),

  // Pre-flight check for material validation before starting
  preflightCheck: (id: string) =>
    fetchApi<import('../types').PreflightCheckResult>(`/print-jobs/${id}/preflight`),

  // Mark a failed job as scrap (no retry intended)
  markAsScrap: (id: string, request?: import('../types').ScrapRequest) =>
    fetchApi<import('../types').PrintJob>(`/print-jobs/${id}/scrap`, {
      method: 'POST',
      body: JSON.stringify(request || {}),
    }),

  // Jobs by recipe
  listByRecipe: (recipeId: string) =>
    fetchApi<import('../types').PrintJob[]>(`/templates/${recipeId}/jobs`),
}

// Queue API
export const gcodeLibraryApi = {
  list: (params?: { q?: string; material?: string; profile?: string; nozzle?: string; layer?: string; time_bucket?: string; usage?: string; sort?: string; page?: number; page_size?: number }) => {
    const searchParams = new URLSearchParams()
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== '') searchParams.set(key, String(value))
      })
    }
    const query = searchParams.toString()
    return fetchApi<import('../types').GCodeLibraryResponse>(`/gcode-library${query ? `?${query}` : ''}`)
  },

  upload: async (file: File, parentSTLId?: string | null) => {
    const formData = new FormData()
    formData.append('file', file)
    if (parentSTLId) formData.append('parent_stl_id', parentSTLId)
    const response = await fetch(`${API_URL}/api/gcode-library/upload`, { method: 'POST', body: formData })
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Upload failed' }))
      throw new Error(error.error)
    }
    return response.json() as Promise<import('../types').GCodeLibraryFile>
  },

  update: (id: string, data: Partial<import('../types').GCodeLibraryFile>) =>
    fetchApi<import('../types').GCodeLibraryFile>(`/gcode-library/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) => fetchApi<void>(`/gcode-library/${id}`, { method: 'DELETE' }),

  setParentSTL: (id: string, parentSTLId: string | null) =>
    fetchApi<void>(`/gcode-library/${id}/parent-stl`, { method: 'PATCH', body: JSON.stringify({ parent_stl_id: parentSTLId }) }),

  setDefaultForSTL: (id: string) =>
    fetchApi<void>(`/gcode-library/${id}/default-for-stl`, { method: 'PATCH' }),

  addToQueue: (id: string, data?: { assigned_spool_id?: string; project_id?: string; material_type?: string; material_color?: string; filament_name?: string; source_type?: 'library' | 'project' }) =>
    fetchApi<import('../types').GCodeQueueItem>(`/gcode-library/${id}/add-to-queue`, { method: 'POST', body: data ? JSON.stringify(data) : undefined }),

  listTags: () => fetchApi<import('../types').Tag[]>('/gcode-library/tags'),

  createTag: (data: { name: string; color?: string }) =>
    fetchApi<import('../types').Tag>('/gcode-library/tags', { method: 'POST', body: JSON.stringify(data) }),

  addTag: (fileId: string, tagId: string) =>
    fetchApi<void>(`/gcode-library/${fileId}/tags/${tagId}`, { method: 'POST' }),

  removeTag: (fileId: string, tagId: string) =>
    fetchApi<void>(`/gcode-library/${fileId}/tags/${tagId}`, { method: 'DELETE' }),
}

export const fileLibraryApi = {
  get: () => fetchApi<import('../types').FileLibraryResponse>('/file-library'),
}

export const stlLibraryApi = {
  list: (params?: { q?: string; sort?: string; page?: number; page_size?: number }) => {
    const searchParams = new URLSearchParams()
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== '') searchParams.set(key, String(value))
      })
    }
    const query = searchParams.toString()
    return fetchApi<import('../types').STLLibraryResponse>(`/stl-library${query ? `?${query}` : ''}`)
  },

  upload: async (file: File, thumbnail?: Blob | null) => {
    const formData = new FormData()
    formData.append('file', file)
    if (thumbnail) formData.append('thumbnail', thumbnail, `${file.name}.png`)
    const response = await fetch(`${API_URL}/api/stl-library/upload`, { method: 'POST', body: formData })
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Upload failed' }))
      throw new Error(error.error)
    }
    return response.json() as Promise<import('../types').STLLibraryFile>
  },

  update: (id: string, data: Partial<import('../types').STLLibraryFile>) =>
    fetchApi<import('../types').STLLibraryFile>(`/stl-library/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),

  updateThumbnail: async (id: string, thumbnail: Blob) => {
    const formData = new FormData()
    formData.append('thumbnail', thumbnail, `${id}.png`)
    const response = await fetch(`${API_URL}/api/stl-library/${id}/thumbnail`, { method: 'POST', body: formData })
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Thumbnail upload failed' }))
      throw new Error(error.error)
    }
    return response.json() as Promise<import('../types').STLLibraryFile>
  },

  addTag: (fileId: string, tagId: string) =>
    fetchApi<void>(`/stl-library/${fileId}/tags`, { method: 'POST', body: JSON.stringify({ tag_id: tagId }) }),

  removeTag: (fileId: string, tagId: string) =>
    fetchApi<void>(`/stl-library/${fileId}/tags/${tagId}`, { method: 'DELETE' }),

  delete: (id: string) => fetchApi<void>(`/stl-library/${id}`, { method: 'DELETE' }),
}

export const queueApi = {
  get: () => fetchApi<import('../types').QueueResponse>('/queue'),

  upload: async (file: File, data: { display_name?: string; notes?: string; printer_id?: string; spool_id?: string; material_type?: string; material_color?: string }) => {
    const formData = new FormData()
    formData.append('file', file)
    Object.entries(data).forEach(([key, value]) => {
      if (value) formData.append(key, value)
    })
    const response = await fetch(`${API_URL}/api/queue/upload`, { method: 'POST', body: formData })
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Upload failed' }))
      throw new Error(error.error)
    }
    return response.json() as Promise<import('../types').GCodeQueueItem>
  },

  fromPrintJob: (jobId: string, data: Partial<import('../types').GCodeQueueItem>) =>
    fetchApi<import('../types').GCodeQueueItem>(`/queue/from-print-job/${jobId}`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').GCodeQueueItem>) =>
    fetchApi<import('../types').GCodeQueueItem>(`/queue/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) => fetchApi<void>(`/queue/${id}`, { method: 'DELETE' }),
  preflight: (id: string) => fetchApi<import('../types').PreflightCheckResult>(`/queue/${id}/preflight`, { method: 'POST' }),
  start: (id: string) => fetchApi<{ status: string }>(`/queue/${id}/start`, { method: 'POST' }),
  setStatus: (id: string, status: import('../types').QueueItemStatus) =>
    fetchApi<{ status: string }>(`/queue/${id}/status`, {
      method: 'POST',
      body: JSON.stringify({ status }),
    }),
  updatePriority: (id: string, priority: number) =>
    fetchApi<{ status: string }>(`/queue/${id}/priority`, {
      method: 'PATCH',
      body: JSON.stringify({ priority }),
    }),
}

// Expenses API
export const expensesApi = {
  list: (status?: string) =>
    fetchApi<import('../types').Expense[]>(
      `/expenses${status ? `?status=${status}` : ''}`
    ),

  get: (id: string) =>
    fetchApi<import('../types').Expense>(`/expenses/${id}`),

  uploadReceipt: async (file: File) => {
    const formData = new FormData()
    formData.append('file', file)

    const response = await fetch(`${API_URL}/api/expenses/receipt`, {
      method: 'POST',
      body: formData,
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: 'Upload failed' }))
      throw new Error(error.error)
    }

    return response.json() as Promise<import('../types').Expense>
  },

  confirm: (
    id: string,
    items: Array<{
      item_id: string
      create_spool: boolean
      material_id?: string
      new_material?: Partial<import('../types').Material>
      weight_grams?: number
    }>
  ) =>
    fetchApi<import('../types').Expense>(`/expenses/${id}/confirm`, {
      method: 'POST',
      body: JSON.stringify({ items }),
    }),

  retry: (id: string) =>
    fetchApi<import('../types').Expense>(`/expenses/${id}/retry`, {
      method: 'POST',
    }),

  delete: (id: string) =>
    fetchApi<void>(`/expenses/${id}`, { method: 'DELETE' }),
}

// Settings API
export interface AppSetting {
  key: string
  value: string
  updated_at: string
}

export const settingsApi = {
  list: () =>
    fetchApi<AppSetting[]>('/settings'),

  get: (key: string) =>
    fetchApi<AppSetting>(`/settings/${key}`),

  set: (key: string, value: string) =>
    fetchApi<{ status: string }>(`/settings/${key}`, {
      method: 'PUT',
      body: JSON.stringify({ value }),
    }),

  delete: (key: string) =>
    fetchApi<void>(`/settings/${key}`, { method: 'DELETE' }),
}

// Backups API
export const backupsApi = {
  list: () =>
    fetchApi<import('../types').BackupInfo[]>('/backups'),

  create: () =>
    fetchApi<import('../types').BackupInfo>('/backups', { method: 'POST' }),

  delete: (name: string) =>
    fetchApi<void>(`/backups/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  restore: (name: string) =>
    fetchApi<{ message: string }>(`/backups/${encodeURIComponent(name)}/restore`, { method: 'POST' }),

  getConfig: () =>
    fetchApi<import('../types').BackupConfig>('/backups/config'),

  updateConfig: (config: import('../types').BackupConfig) =>
    fetchApi<import('../types').BackupConfig>('/backups/config', {
      method: 'PUT',
      body: JSON.stringify(config),
    }),
}

// Sales API
export const salesApi = {
  list: (projectId?: string) =>
    fetchApi<import('../types').Sale[]>(
      `/sales${projectId ? `?project_id=${projectId}` : ''}`
    ),

  get: (id: string) =>
    fetchApi<import('../types').Sale>(`/sales/${id}`),

  create: (data: Partial<import('../types').Sale>) =>
    fetchApi<import('../types').Sale>('/sales', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Sale>) =>
    fetchApi<import('../types').Sale>(`/sales/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/sales/${id}`, { method: 'DELETE' }),

  getWeeklyInsights: () =>
    fetchApi<import('../types').WeeklyInsights>('/sales/weekly-insights'),
}

// Financial Summary
export interface FinancialSummary {
  total_expenses_cents: number
  total_sales_gross_cents: number
  total_sales_net_cents: number
  total_fees_cents: number
  total_material_cost: number
  total_material_used_grams: number
  total_cogs_cents: number
  net_profit_cents: number
  confirmed_expense_count: number
  pending_expense_count: number
  sales_count: number
  completed_print_count: number
  successful_print_count: number
}

// Stats API
export const statsApi = {
  getFinancialSummary: (period?: string) =>
    fetchApi<FinancialSummary>(`/stats/financial${period ? `?period=${period}` : ''}`),

  getTimeSeries: (period: string) =>
    fetchApi<import('../types').TimeSeriesData>(`/stats/time-series?period=${period}`),

  getExpensesByCategory: (period: string) =>
    fetchApi<import('../types').CategoryBreakdown[]>(`/stats/expenses-by-category?period=${period}`),

  getSalesByChannel: (period: string) =>
    fetchApi<import('../types').ChannelBreakdown[]>(`/stats/sales-by-channel?period=${period}`),

  getSalesByProject: () =>
    fetchApi<import('../types').ProjectSales[]>('/stats/sales-by-project'),

  getUsage: (period?: string) => fetchApi<Record<string, number>>(`/stats/usage${period ? `?period=${period}` : ''}`),
}

// Templates (Recipes) API
export const templatesApi = {
  list: (activeOnly?: boolean) =>
    fetchApi<import('../types').Template[]>(
      `/templates${activeOnly ? '?active=true' : ''}`
    ),

  get: (id: string) =>
    fetchApi<import('../types').Template>(`/templates/${id}`),

  create: (data: Partial<import('../types').Template>) =>
    fetchApi<import('../types').Template>('/templates', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Template>) =>
    fetchApi<import('../types').Template>(`/templates/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/templates/${id}`, { method: 'DELETE' }),

  addDesign: (
    id: string,
    data: { design_id: string; quantity: number; is_primary: boolean; notes?: string }
  ) =>
    fetchApi<import('../types').TemplateDesign>(`/templates/${id}/designs`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  removeDesign: (id: string, designId: string) =>
    fetchApi<void>(`/templates/${id}/designs/${designId}`, { method: 'DELETE' }),

  instantiate: (
    id: string,
    opts: {
      order_quantity?: number
      customer_notes?: string
      external_order_id?: string
      source?: string
      material_spool_id?: string
    }
  ) =>
    fetchApi<{ project: import('../types').Project; jobs: import('../types').PrintJob[] }>(
      `/templates/${id}/instantiate`,
      {
        method: 'POST',
        body: JSON.stringify(opts),
      }
    ),

  // Recipe material methods
  listMaterials: (id: string) =>
    fetchApi<import('../types').RecipeMaterial[]>(`/templates/${id}/materials`),

  addMaterial: (
    id: string,
    data: {
      material_type: string
      color_spec?: import('../types').ColorSpec
      weight_grams: number
      ams_position?: number
      sequence_order: number
      notes?: string
    }
  ) =>
    fetchApi<import('../types').RecipeMaterial>(`/templates/${id}/materials`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateMaterial: (id: string, materialId: string, data: Partial<import('../types').RecipeMaterial>) =>
    fetchApi<import('../types').RecipeMaterial>(`/templates/${id}/materials/${materialId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  removeMaterial: (id: string, materialId: string) =>
    fetchApi<void>(`/templates/${id}/materials/${materialId}`, { method: 'DELETE' }),

  // Recipe compatibility methods
  getCompatiblePrinters: (id: string) =>
    fetchApi<import('../types').Printer[]>(`/templates/${id}/compatible-printers`),

  getCompatibleSpools: (id: string) =>
    fetchApi<import('../types').CompatibleSpool[]>(`/templates/${id}/compatible-spools`),

  getCostEstimate: (id: string) =>
    fetchApi<import('../types').RecipeCostEstimate>(`/templates/${id}/cost-estimate`),

  validatePrinter: (id: string, printerId: string) =>
    fetchApi<import('../types').PrinterValidationResult>(`/templates/${id}/validate-printer/${printerId}`, {
      method: 'POST',
    }),

  // Recipe supply methods
  listSupplies: (id: string) =>
    fetchApi<import('../types').RecipeSupply[]>(`/templates/${id}/supplies`),

  addSupply: (
    id: string,
    data: {
      name: string
      unit_cost_cents: number
      quantity: number
      notes?: string
      material_id?: string
    }
  ) =>
    fetchApi<import('../types').RecipeSupply>(`/templates/${id}/supplies`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateSupply: (id: string, supplyId: string, data: Partial<import('../types').RecipeSupply>) =>
    fetchApi<import('../types').RecipeSupply>(`/templates/${id}/supplies/${supplyId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  removeSupply: (id: string, supplyId: string) =>
    fetchApi<void>(`/templates/${id}/supplies/${supplyId}`, { method: 'DELETE' }),

  // Analytics
  getAnalytics: (id: string) =>
    fetchApi<import('../types').TemplateAnalytics>(`/templates/${id}/analytics`),
}

// Etsy API
export const etsyApi = {
  configure: (data: { client_id: string; redirect_uri?: string }) =>
    fetchApi<{ status: string }>('/integrations/etsy/configure', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  getAuthUrl: () => fetchApi<{ url: string }>('/integrations/etsy/auth'),

  getStatus: () => fetchApi<import('../types').EtsyIntegration>('/integrations/etsy/status'),

  disconnect: () =>
    fetchApi<{ status: string }>('/integrations/etsy/disconnect', { method: 'POST' }),

  // Receipts/Orders
  syncReceipts: () =>
    fetchApi<import('../types').SyncResult>('/integrations/etsy/receipts/sync', { method: 'POST' }),

  listReceipts: (params?: { processed?: boolean; limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.processed !== undefined) searchParams.set('processed', String(params.processed))
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').EtsyReceipt[]>(`/integrations/etsy/receipts${query ? `?${query}` : ''}`)
  },

  getReceipt: (id: string) =>
    fetchApi<import('../types').EtsyReceipt>(`/integrations/etsy/receipts/${id}`),

  processReceipt: (id: string) =>
    fetchApi<{ project: import('../types').Project }>(`/integrations/etsy/receipts/${id}/process`, {
      method: 'POST',
    }),

  // Listings
  syncListings: () =>
    fetchApi<import('../types').SyncResult>('/integrations/etsy/listings/sync', { method: 'POST' }),

  listListings: (params?: { state?: string; limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.state) searchParams.set('state', params.state)
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').EtsyListing[]>(`/integrations/etsy/listings${query ? `?${query}` : ''}`)
  },

  getListing: (id: string) =>
    fetchApi<import('../types').EtsyListing>(`/integrations/etsy/listings/${id}`),

  linkListing: (id: string, data: { template_id: string; sku?: string; sync_inventory?: boolean }) =>
    fetchApi<{ status: string }>(`/integrations/etsy/listings/${id}/link`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  unlinkListing: (id: string, templateId: string) =>
    fetchApi<void>(`/integrations/etsy/listings/${id}/link?template_id=${templateId}`, {
      method: 'DELETE',
    }),

  syncInventory: (id: string) =>
    fetchApi<{ status: string }>(`/integrations/etsy/listings/${id}/sync-inventory`, {
      method: 'POST',
    }),

  // Webhook Events
  listWebhookEvents: (params?: { type?: string; limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.type) searchParams.set('type', params.type)
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').EtsyWebhookEvent[]>(`/integrations/etsy/webhook/events${query ? `?${query}` : ''}`)
  },

  reprocessWebhookEvent: (id: string) =>
    fetchApi<{ status: string }>(`/integrations/etsy/webhook/events/${id}/reprocess`, {
      method: 'POST',
    }),
}

// Bambu Cloud API
export const bambuCloudApi = {
  login: (email: string, password: string) =>
    fetchApi<{ status: string }>('/bambu-cloud/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  verify: (email: string, code: string) =>
    fetchApi<{ status: string }>('/bambu-cloud/verify', {
      method: 'POST',
      body: JSON.stringify({ email, code }),
    }),

  status: () =>
    fetchApi<import('../types').BambuCloudStatus>('/bambu-cloud/status'),

  devices: () =>
    fetchApi<import('../types').CloudDevice[]>('/bambu-cloud/devices'),

  addDevice: (devId: string) =>
    fetchApi<import('../types').Printer>('/bambu-cloud/devices/add', {
      method: 'POST',
      body: JSON.stringify({ dev_id: devId }),
    }),

  logout: () =>
    fetchApi<void>('/bambu-cloud/logout', { method: 'DELETE' }),
}

// Squarespace API
export const squarespaceApi = {
  // Connection
  connect: (apiKey: string) =>
    fetchApi<import('../types').SquarespaceIntegration>('/integrations/squarespace/connect', {
      method: 'POST',
      body: JSON.stringify({ api_key: apiKey })
    }),

  getStatus: () =>
    fetchApi<import('../types').SquarespaceIntegration>('/integrations/squarespace/status'),

  disconnect: () =>
    fetchApi<{ status: string }>('/integrations/squarespace/disconnect', { method: 'POST' }),

  // Orders
  syncOrders: () =>
    fetchApi<import('../types').SyncResult>('/integrations/squarespace/orders/sync', { method: 'POST' }),

  listOrders: (params?: { processed?: boolean; limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.processed !== undefined) searchParams.set('processed', String(params.processed))
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').SquarespaceOrder[]>(`/integrations/squarespace/orders${query ? `?${query}` : ''}`)
  },

  getOrder: (id: string) =>
    fetchApi<import('../types').SquarespaceOrder>(`/integrations/squarespace/orders/${id}`),

  processOrder: (id: string) =>
    fetchApi<{ project_id: string; project: import('../types').Project }>(`/integrations/squarespace/orders/${id}/process`, {
      method: 'POST',
    }),

  // Products
  syncProducts: () =>
    fetchApi<import('../types').SyncResult>('/integrations/squarespace/products/sync', { method: 'POST' }),

  listProducts: (params?: { limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').SquarespaceProduct[]>(`/integrations/squarespace/products${query ? `?${query}` : ''}`)
  },

  getProduct: (id: string) =>
    fetchApi<import('../types').SquarespaceProduct>(`/integrations/squarespace/products/${id}`),

  linkProduct: (id: string, templateId: string, sku?: string) =>
    fetchApi<{ status: string }>(`/integrations/squarespace/products/${id}/link`, {
      method: 'POST',
      body: JSON.stringify({ template_id: templateId, sku })
    }),

  unlinkProduct: (id: string, templateId: string) =>
    fetchApi<void>(`/integrations/squarespace/products/${id}/link?template_id=${templateId}`, {
      method: 'DELETE'
    }),
}

// Dispatch API (auto-dispatch queue management)
export const dispatchApi = {
  listPending: () =>
    fetchApi<import('../types').DispatchRequest[]>('/dispatch/requests'),

  confirm: (id: string) =>
    fetchApi<{ status: string }>(`/dispatch/requests/${id}/confirm`, { method: 'POST' }),

  reject: (id: string, reason?: string) =>
    fetchApi<{ status: string }>(`/dispatch/requests/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),

  skip: (id: string) =>
    fetchApi<{ status: string }>(`/dispatch/requests/${id}/skip`, { method: 'POST' }),

  getGlobalSettings: () =>
    fetchApi<{ enabled: boolean }>('/dispatch/settings'),

  updateGlobalSettings: (enabled: boolean) =>
    fetchApi<{ status: string }>('/dispatch/settings', {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),

  getPrinterSettings: (printerId: string) =>
    fetchApi<import('../types').AutoDispatchSettings>(`/printers/${printerId}/dispatch-settings`),

  updatePrinterSettings: (printerId: string, settings: Partial<import('../types').AutoDispatchSettings>) =>
    fetchApi<import('../types').AutoDispatchSettings>(`/printers/${printerId}/dispatch-settings`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),
}

// Print Jobs API extension for priority
export const printJobPriorityApi = {
  updatePriority: (id: string, priority: number) =>
    fetchApi<{ status: string }>(`/print-jobs/${id}/priority`, {
      method: 'PATCH',
      body: JSON.stringify({ priority }),
    }),
}

// ============================================
// New Feature Gap APIs
// ============================================

// Alerts API
export const alertsApi = {
  list: () =>
    fetchApi<import('../types').Alert[]>('/alerts'),

  getCounts: () =>
    fetchApi<import('../types').AlertCounts>('/alerts/counts'),

  dismiss: (type: string, entityId: string, duration?: string) =>
    fetchApi<{ status: string }>(`/alerts/${type}/${entityId}/dismiss`, {
      method: 'POST',
      body: JSON.stringify({ duration: duration || '1h' }),
    }),

  undismiss: (type: string, entityId: string) =>
    fetchApi<{ status: string }>(`/alerts/${type}/${entityId}/dismiss`, {
      method: 'DELETE',
    }),

  updateMaterialThreshold: (materialId: string, thresholdGrams: number) =>
    fetchApi<{ status: string }>(`/materials/${materialId}/threshold`, {
      method: 'PATCH',
      body: JSON.stringify({ threshold_grams: thresholdGrams }),
    }),
}

// Orders API (Unified)
export const ordersApi = {
  list: (params?: { status?: string; source?: string; limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.status) searchParams.set('status', params.status)
    if (params?.source) searchParams.set('source', params.source)
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').Order[]>(`/orders${query ? `?${query}` : ''}`)
  },

  get: (id: string) =>
    fetchApi<import('../types').Order>(`/orders/${id}`),

  create: (data: { customer_name: string; customer_email?: string; due_date?: string; priority?: number; notes?: string }) =>
    fetchApi<import('../types').Order>('/orders', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Order>) =>
    fetchApi<import('../types').Order>(`/orders/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/orders/${id}`, { method: 'DELETE' }),

  updateStatus: (id: string, status: import('../types').OrderStatus) =>
    fetchApi<import('../types').Order>(`/orders/${id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    }),

  getProgress: (id: string) =>
    fetchApi<import('../types').OrderProgress>(`/orders/${id}/progress`),

  getCounts: () =>
    fetchApi<import('../types').OrderCounts>('/orders/counts'),

  markShipped: (id: string, trackingNumber?: string) =>
    fetchApi<import('../types').Order>(`/orders/${id}/ship`, {
      method: 'POST',
      body: JSON.stringify({ tracking_number: trackingNumber }),
    }),

  // Order items
  addItem: (orderId: string, data: { template_id?: string; sku?: string; quantity: number; notes?: string }) =>
    fetchApi<import('../types').OrderItem>(`/orders/${orderId}/items`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  removeItem: (orderId: string, itemId: string) =>
    fetchApi<void>(`/orders/${orderId}/items/${itemId}`, { method: 'DELETE' }),

  processItem: (orderId: string, itemId: string) =>
    fetchApi<import('../types').Project>(`/orders/${orderId}/items/${itemId}/process`, {
      method: 'POST',
    }),
}

// Tags API
export const tagsApi = {
  list: () =>
    fetchApi<import('../types').Tag[]>('/tags'),

  get: (id: string) =>
    fetchApi<import('../types').Tag>(`/tags/${id}`),

  create: (data: { name: string; color?: string }) =>
    fetchApi<import('../types').Tag>('/tags', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: { name?: string; color?: string }) =>
    fetchApi<import('../types').Tag>(`/tags/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/tags/${id}`, { method: 'DELETE' }),

  // Part tags
  getPartTags: (partId: string) =>
    fetchApi<import('../types').Tag[]>(`/parts/${partId}/tags`),

  addTagToPart: (partId: string, tagId: string) =>
    fetchApi<void>(`/parts/${partId}/tags/${tagId}`, { method: 'POST' }),

  removeTagFromPart: (partId: string, tagId: string) =>
    fetchApi<void>(`/parts/${partId}/tags/${tagId}`, { method: 'DELETE' }),

  // Design tags
  getDesignTags: (designId: string) =>
    fetchApi<import('../types').Tag[]>(`/designs/${designId}/tags`),

  addTagToDesign: (designId: string, tagId: string) =>
    fetchApi<void>(`/designs/${designId}/tags/${tagId}`, { method: 'POST' }),

  removeTagFromDesign: (designId: string, tagId: string) =>
    fetchApi<void>(`/designs/${designId}/tags/${tagId}`, { method: 'DELETE' }),

  // Search by tag
  listPartsByTag: (tagId: string) =>
    fetchApi<import('../types').Part[]>(`/tags/${tagId}/parts`),

  listDesignsByTag: (tagId: string) =>
    fetchApi<import('../types').Design[]>(`/tags/${tagId}/designs`),
}

// Shopify API
export const shopifyApi = {
  getAuthUrl: (shopDomain: string) =>
    fetchApi<{ auth_url: string }>(`/integrations/shopify/auth-url?shop=${encodeURIComponent(shopDomain)}`),

  getStatus: () =>
    fetchApi<import('../types').ShopifyIntegrationStatus>('/integrations/shopify/status'),

  disconnect: () =>
    fetchApi<{ status: string }>('/integrations/shopify', { method: 'DELETE' }),

  syncOrders: () =>
    fetchApi<import('../types').SyncResult>('/integrations/shopify/sync', { method: 'POST' }),

  listOrders: (params?: { processed?: boolean; limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.processed !== undefined) searchParams.set('processed', String(params.processed))
    if (params?.limit) searchParams.set('limit', String(params.limit))
    if (params?.offset) searchParams.set('offset', String(params.offset))
    const query = searchParams.toString()
    return fetchApi<import('../types').ShopifyOrder[]>(`/integrations/shopify/orders${query ? `?${query}` : ''}`)
  },

  getOrder: (id: string) =>
    fetchApi<import('../types').ShopifyOrder>(`/integrations/shopify/orders/${id}`),

  processOrder: (id: string) =>
    fetchApi<import('../types').Order>(`/integrations/shopify/orders/${id}/process`, {
      method: 'POST',
    }),

  linkProduct: (productId: string, templateId: string, sku?: string) =>
    fetchApi<{ status: string }>(`/integrations/shopify/products/${productId}/link`, {
      method: 'POST',
      body: JSON.stringify({ template_id: templateId, sku }),
    }),

  unlinkProduct: (productId: string, templateId: string) =>
    fetchApi<void>(`/integrations/shopify/products/${productId}/link?template_id=${templateId}`, {
      method: 'DELETE',
    }),
}

// Timeline API (Gantt View)
export const timelineApi = {
  getTimeline: (params?: { start?: string; end?: string }) => {
    const searchParams = new URLSearchParams()
    if (params?.start) searchParams.set('start', params.start)
    if (params?.end) searchParams.set('end', params.end)
    const query = searchParams.toString()
    return fetchApi<import('../types').TimelineItem[]>(`/timeline${query ? `?${query}` : ''}`)
  },

  getOrderTimeline: (orderId: string) =>
    fetchApi<import('../types').TimelineItem>(`/timeline/orders/${orderId}`),

  getProjectTimeline: (projectId: string) =>
    fetchApi<import('../types').TimelineItem>(`/timeline/projects/${projectId}`),
}

// WebSocket connection for real-time updates
export function createWebSocket(onMessage: (event: { type: string; data: unknown }) => void) {
  const wsUrl = API_URL.replace('http', 'ws') + '/ws'
  const ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      onMessage(data)
    } catch (e) {
      console.error('Failed to parse WebSocket message:', e)
    }
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }

  return ws
}

// Feedback API
export const feedbackApi = {
  submit: (data: { type: string; message: string; contact?: string; page?: string }) =>
    fetchApi<import('../types').Feedback>('/feedback', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
}

// Customers API
export const customersApi = {
  list: (params?: { search?: string }) => {
    const searchParams = new URLSearchParams()
    if (params?.search) searchParams.set('search', params.search)
    const query = searchParams.toString()
    return fetchApi<import('../types').Customer[]>(`/customers${query ? `?${query}` : ''}`)
  },

  get: (id: string) =>
    fetchApi<import('../types').Customer>(`/customers/${id}`),

  create: (data: { name: string; email?: string; company?: string; phone?: string; notes?: string }) =>
    fetchApi<import('../types').Customer>('/customers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Customer>) =>
    fetchApi<import('../types').Customer>(`/customers/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/customers/${id}`, { method: 'DELETE' }),
}

// Quotes API
export const quotesApi = {
  list: (params?: { status?: string; customer_id?: string }) => {
    const searchParams = new URLSearchParams()
    if (params?.status) searchParams.set('status', params.status)
    if (params?.customer_id) searchParams.set('customer_id', params.customer_id)
    const query = searchParams.toString()
    return fetchApi<import('../types').Quote[]>(`/quotes${query ? `?${query}` : ''}`)
  },

  get: (id: string) =>
    fetchApi<import('../types').Quote>(`/quotes/${id}`),

  create: (data: { customer_id: string; title: string; notes?: string; valid_until?: string }) =>
    fetchApi<import('../types').Quote>('/quotes', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<import('../types').Quote>) =>
    fetchApi<import('../types').Quote>(`/quotes/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchApi<void>(`/quotes/${id}`, { method: 'DELETE' }),

  send: (id: string) =>
    fetchApi<import('../types').Quote>(`/quotes/${id}/send`, { method: 'POST' }),

  accept: (id: string, optionId: string) =>
    fetchApi<import('../types').Quote>(`/quotes/${id}/accept`, {
      method: 'POST',
      body: JSON.stringify({ option_id: optionId }),
    }),

  reject: (id: string) =>
    fetchApi<import('../types').Quote>(`/quotes/${id}/reject`, { method: 'POST' }),

  // Options
  createOption: (quoteId: string, data: { name: string; description?: string; sort_order?: number }) =>
    fetchApi<import('../types').QuoteOption>(`/quotes/${quoteId}/options`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateOption: (quoteId: string, optionId: string, data: Partial<import('../types').QuoteOption>) =>
    fetchApi<import('../types').QuoteOption>(`/quotes/${quoteId}/options/${optionId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteOption: (quoteId: string, optionId: string) =>
    fetchApi<void>(`/quotes/${quoteId}/options/${optionId}`, { method: 'DELETE' }),

  // Line items
  createLineItem: (quoteId: string, optionId: string, data: {
    type: string; description: string; quantity: number; unit: string;
    unit_price_cents: number; total_cents: number; sort_order?: number; project_id?: string
  }) =>
    fetchApi<import('../types').QuoteLineItem>(`/quotes/${quoteId}/options/${optionId}/items`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateLineItem: (quoteId: string, optionId: string, itemId: string, data: Partial<import('../types').QuoteLineItem>) =>
    fetchApi<import('../types').QuoteLineItem>(`/quotes/${quoteId}/options/${optionId}/items/${itemId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  deleteLineItem: (quoteId: string, optionId: string, itemId: string) =>
    fetchApi<void>(`/quotes/${quoteId}/options/${optionId}/items/${itemId}`, { method: 'DELETE' }),
}

// Public API (no auth, for shareable pages)
const PUBLIC_API_URL = import.meta.env.VITE_API_URL ?? (typeof window !== 'undefined' ? `${window.location.protocol}//${window.location.host}` : 'http://localhost:8084')

async function fetchPublicApi<T>(path: string): Promise<T> {
  const url = `${PUBLIC_API_URL}/api/public${path}`
  const response = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
  })
  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error(error.error || `HTTP ${response.status}`)
  }
  return response.json() as Promise<T>
}

export const publicApi = {
  getQuote: (token: string) =>
    fetchPublicApi<import('../types').Quote>(`/quotes/${token}`),

  getBusinessInfo: () =>
    fetchPublicApi<Record<string, string>>(`/business-info`),
}

