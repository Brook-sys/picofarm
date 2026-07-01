import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Plus, Package, Droplet, ShoppingCart, Trash2, Edit2, Check, X } from 'lucide-react'
import { materialsApi, spoolsApi } from '../api/client'
import AppToast, { type AppToastState } from '../components/AppToast'
import { cn, getStatusBadge } from '../lib/utils'
import type { Material, MaterialSpool, MaterialType } from '../types'

export default function Materials() {
  const queryClient = useQueryClient()

  const { data: materials = [], isLoading: materialsLoading } = useQuery({
    queryKey: ['materials'],
    queryFn: () => materialsApi.list(),
    refetchInterval: 5000,
  })

  const { data: spools = [], isLoading: spoolsLoading } = useQuery({
    queryKey: ['spools'],
    queryFn: () => spoolsApi.list(),
  })

  const filamentMaterials = materials.filter(m => m.type !== 'supply')
  const supplyMaterials = materials.filter(m => m.type === 'supply')

  const createMaterial = useMutation({
    mutationFn: (data: Partial<Material>) => materialsApi.create(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['materials'] }),
  })

  const createSpool = useMutation({
    mutationFn: (data: Partial<MaterialSpool>) => spoolsApi.create(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['spools'] }),
  })

  const updateMaterial = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Material> }) => materialsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['materials'] })
      queryClient.invalidateQueries({ queryKey: ['spools'] })
    },
  })

  const updateSpool = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<MaterialSpool> }) => spoolsApi.update(id, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['spools'] }),
  })

  const deleteMaterial = useMutation({
    mutationFn: (id: string) => materialsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['materials'] })
      queryClient.invalidateQueries({ queryKey: ['materials', 'supply'] })
    },
  })

  const deleteSpool = useMutation({
    mutationFn: (id: string) => spoolsApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['spools'] }),
  })

  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)
  const [materialError, setMaterialError] = useState('')
  const [toast, setToast] = useState<AppToastState | null>(null)

  const showToast = (next: AppToastState) => {
    setToast(next)
    window.setTimeout(() => setToast(null), 3500)
  }

  const handleDeleteMaterial = async (material: Material) => {
    if (confirmDeleteId === material.id) {
      setMaterialError('')
      try {
        await deleteMaterial.mutateAsync(material.id)
        setConfirmDeleteId(null)
        showToast({ title: 'Material deleted', message: material.name, tone: 'success' })
      } catch (err) {
        setMaterialError(err instanceof Error ? err.message : 'Failed to delete material')
      }
    } else {
      setConfirmDeleteId(material.id)
    }
  }

  const handleDeleteSpool = async (spool: MaterialSpool) => {
    if (confirmDeleteId === spool.id) {
      setMaterialError('')
      try {
        await deleteSpool.mutateAsync(spool.id)
        setConfirmDeleteId(null)
        showToast({ title: 'Spool deleted', message: 'Inventory references were updated.', tone: 'success' })
      } catch (err) {
        setMaterialError(err instanceof Error ? err.message : 'Failed to delete spool')
      }
    } else {
      setConfirmDeleteId(spool.id)
    }
  }

  const [showAddMaterial, setShowAddMaterial] = useState(false)
  const [showAddSpool, setShowAddSpool] = useState(false)
  const [tab, setTab] = useState<'spools' | 'catalog' | 'supplies'>('spools')
  const [editingSpoolId, setEditingSpoolId] = useState<string | null>(null)
  const [spoolWeight, setSpoolWeight] = useState('')
  const [spoolForm, setSpoolForm] = useState({ location: '', notes: '' })
  const [editingMaterialId, setEditingMaterialId] = useState<string | null>(null)
  const [materialForm, setMaterialForm] = useState({ name: '', manufacturer: '', color: '', color_hex: '', cost_per_kg: '' })

  const handleCreateMaterial = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    
    await createMaterial.mutateAsync({
      name: formData.get('name') as string,
      type: formData.get('type') as MaterialType,
      manufacturer: formData.get('manufacturer') as string,
      color: formData.get('color') as string,
      color_hex: formData.get('color_hex') as string,
      cost_per_kg: parseFloat(formData.get('cost_per_kg') as string) || 0,
    })
    
    setShowAddMaterial(false)
    showToast({ title: 'Material added', message: formData.get('name') as string, tone: 'success' })
  }

  const handleCreateSpool = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const formData = new FormData(e.currentTarget)
    
    await createSpool.mutateAsync({
      material_id: formData.get('material_id') as string,
      initial_weight: parseFloat(formData.get('initial_weight') as string) || 1000,
      remaining_weight: parseFloat(formData.get('initial_weight') as string) || 1000,
      purchase_cost: 0,
      location: formData.get('location') as string,
    })
    
    setShowAddSpool(false)
    showToast({ title: 'Spool added', message: 'Filament inventory updated.', tone: 'success' })
  }

  const getMaterialById = (id: string) => materials.find(m => m.id === id)

  const setClampedSpoolWeight = (value: string) => {
    const n = Math.max(0, Math.min(1000, parseFloat(value) || 0))
    setSpoolWeight(String(Math.round(n)))
  }

  const startEditMaterial = (material: Material) => {
    setEditingMaterialId(material.id)
    setMaterialForm({
      name: material.name || '',
      manufacturer: material.manufacturer || '',
      color: material.color || '',
      color_hex: material.color_hex || '',
      cost_per_kg: String(material.cost_per_kg || 0),
    })
  }

  const saveMaterial = async (material: Material) => {
    await updateMaterial.mutateAsync({
      id: material.id,
      data: {
        ...material,
        name: materialForm.name,
        manufacturer: materialForm.manufacturer,
        color: materialForm.color,
        color_hex: materialForm.color_hex.toUpperCase(),
        cost_per_kg: parseFloat(materialForm.cost_per_kg) || 0,
      },
    })
    setEditingMaterialId(null)
  }

  const startEditSpool = (spool: MaterialSpool) => {
    setEditingSpoolId(spool.id)
    setSpoolWeight(String(spool.remaining_weight.toFixed(0)))
    setSpoolForm({
      location: spool.location || '',
      notes: spool.notes || '',
    })
  }

  const saveSpoolWeight = async (spool: MaterialSpool) => {
    const remaining = Math.max(0, Math.min(1000, parseFloat(spoolWeight)))
    if (Number.isNaN(remaining)) return
    await updateSpool.mutateAsync({
      id: spool.id,
      data: {
        ...spool,
        remaining_weight: Math.min(remaining, 1000),
        location: spoolForm.location,
        notes: spoolForm.notes,
      },
    })
    setEditingSpoolId(null)
  }

  return (
    <div className="p-4 sm:p-6 lg:p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-display font-bold text-surface-100">
            Materials
          </h1>
          <p className="text-surface-400 mt-1">
            Manage your filament inventory and supplies
          </p>
        </div>
        <div className="flex gap-2">
          <button 
            onClick={() => setShowAddMaterial(true)}
            className="btn btn-secondary"
          >
            <Plus className="h-4 w-4 mr-2" />
            Add Material
          </button>
          <button 
            onClick={() => setShowAddSpool(true)}
            className="btn btn-primary"
          >
            <Plus className="h-4 w-4 mr-2" />
            Add Spool
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-2 mb-6">
        <button
          onClick={() => setTab('spools')}
          className={cn(
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            tab === 'spools'
              ? 'bg-accent-500/20 text-accent-400'
              : 'text-surface-400 hover:text-surface-100 hover:bg-surface-800'
          )}
        >
          <Droplet className="h-4 w-4 inline mr-2" />
          Inventory ({spools.length})
        </button>
        <button
          onClick={() => setTab('catalog')}
          className={cn(
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            tab === 'catalog'
              ? 'bg-accent-500/20 text-accent-400'
              : 'text-surface-400 hover:text-surface-100 hover:bg-surface-800'
          )}
        >
          <Package className="h-4 w-4 inline mr-2" />
          Catalog ({filamentMaterials.length})
        </button>
        <button
          onClick={() => setTab('supplies')}
          className={cn(
            'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
            tab === 'supplies'
              ? 'bg-accent-500/20 text-accent-400'
              : 'text-surface-400 hover:text-surface-100 hover:bg-surface-800'
          )}
        >
          <ShoppingCart className="h-4 w-4 inline mr-2" />
          Supplies ({supplyMaterials.length})
        </button>
      </div>

      {materialError && <div className="mb-4 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">{materialError}</div>}
      {toast && <AppToast toast={toast} onClose={() => setToast(null)} />}

      {/* Spools Tab */}
      {tab === 'spools' && (
        spoolsLoading ? (
          <div className="text-surface-500">Loading...</div>
        ) : spools.length === 0 ? (
          <div className="text-center py-16">
            <Droplet className="h-16 w-16 mx-auto mb-4 text-surface-600" />
            <h3 className="text-xl font-semibold text-surface-300 mb-2">
              No spools in inventory
            </h3>
            <p className="text-surface-500 mb-4">
              Add your first spool to start tracking material usage
            </p>
            <button 
              onClick={() => setShowAddSpool(true)}
              className="btn btn-primary"
            >
              <Plus className="h-4 w-4 mr-2" />
              Add Spool
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            {spools.map((spool) => {
              const material = getMaterialById(spool.material_id)
              const percentRemaining = (spool.remaining_weight / spool.initial_weight) * 100
              
              return (
                <div key={spool.id} className="card p-4">
                  <div className="flex items-center gap-3 mb-3">
                    <div
                      className="w-8 h-8 rounded-full border-2 border-surface-700"
                      style={{ backgroundColor: material?.color_hex || '#666' }}
                    />
                    <div className="flex-1 min-w-0">
                      <h3 className="font-medium text-surface-100 truncate">
                        {material?.name || 'Unknown'}
                      </h3>
                      <p className="text-xs text-surface-500">
                        {material?.type?.toUpperCase()}
                      </p>
                    </div>
                    {confirmDeleteId === spool.id ? (
                      <button
                        onClick={() => handleDeleteSpool(spool)}
                        onBlur={() => setConfirmDeleteId(null)}
                        className="text-xs px-2 py-1 rounded bg-red-500/20 text-red-400 border border-red-500/50 hover:bg-red-500/30 transition-colors"
                        autoFocus
                      >
                        Delete?
                      </button>
                    ) : (
                      <button
                        onClick={() => handleDeleteSpool(spool)}
                        className="p-1.5 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                        title="Delete spool"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    )}
                  </div>
                  
                  <div className="space-y-2">
                    <div className="flex items-center justify-between text-sm gap-2">
                      <span className="text-surface-500">Remaining</span>
                      {editingSpoolId === spool.id ? (
                        <div className="flex items-center gap-1">
                          <div className="relative">
                            <input
                              value={spoolWeight}
                              onChange={e => setClampedSpoolWeight(e.target.value)}
                              className="input w-24 h-8 pr-7 text-xs text-right"
                              type="number"
                              min="0"
                              max="1000"
                            />
                            <span className="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] text-surface-500">g</span>
                          </div>
                          <button onClick={() => saveSpoolWeight(spool)} className="p-1 text-emerald-400 hover:bg-emerald-500/10 rounded"><Check className="h-3.5 w-3.5" /></button>
                          <button onClick={() => setEditingSpoolId(null)} className="p-1 text-surface-400 hover:bg-surface-800 rounded"><X className="h-3.5 w-3.5" /></button>
                        </div>
                      ) : (
                        <button onClick={() => startEditSpool(spool)} className="text-surface-300 hover:text-accent-400 inline-flex items-center gap-1">
                          {spool.remaining_weight.toFixed(0)}g / {spool.initial_weight.toFixed(0)}g <Edit2 className="h-3 w-3" />
                        </button>
                      )}
                    </div>
                    {editingSpoolId === spool.id ? (
                      <div className="space-y-2" title={`${spoolWeight || 0}g remaining`}>
                        <div className="relative h-7 flex items-center">
                          <div className="absolute left-0 right-0 h-2 rounded-full bg-surface-800 overflow-hidden">
                            <div
                              className={cn(
                                'h-full transition-all',
                                (parseFloat(spoolWeight) || 0) / spool.initial_weight * 100 > 30 ? 'bg-emerald-500' :
                                (parseFloat(spoolWeight) || 0) / spool.initial_weight * 100 > 10 ? 'bg-amber-500' :
                                'bg-red-500'
                              )}
                              style={{ width: `${Math.min(((parseFloat(spoolWeight) || 0) / spool.initial_weight) * 100, 100)}%` }}
                            />
                          </div>
                          <input
                            type="range"
                            min="0"
                            max="1000"
                            step="1"
                            value={spoolWeight || '0'}
                            onChange={e => setClampedSpoolWeight(e.target.value)}
                            className="relative z-10 w-full cursor-pointer bg-transparent accent-accent-500"
                          />
                        </div>
                        <div className="flex justify-between text-[10px] text-surface-500">
                          <span>0g</span>
                          <span>{spoolWeight || 0}g</span>
                          <span>1000g</span>
                        </div>
                      </div>
                    ) : (
                      <div className="h-2 bg-surface-800 rounded-full overflow-hidden" title={`${spool.remaining_weight.toFixed(0)}g remaining`}>
                        <div
                          className={cn(
                            'h-full transition-all',
                            percentRemaining > 30 ? 'bg-emerald-500' :
                            percentRemaining > 10 ? 'bg-amber-500' :
                            'bg-red-500'
                          )}
                          style={{ width: `${percentRemaining}%` }}
                        />
                      </div>
                    )}
                    {editingSpoolId === spool.id && (
                      <div className="space-y-2 rounded-lg border border-surface-800 bg-surface-900/40 p-2">
                        <label className="text-[10px] text-surface-500 block">
                          Location
                          <input
                            value={spoolForm.location}
                            onChange={e => setSpoolForm(prev => ({ ...prev, location: e.target.value }))}
                            className="input mt-1 h-8 text-xs"
                            placeholder="Dry Box A, Shelf 3"
                          />
                        </label>
                        <label className="text-[10px] text-surface-500 block">
                          Notes
                          <textarea
                            value={spoolForm.notes}
                            onChange={e => setSpoolForm(prev => ({ ...prev, notes: e.target.value }))}
                            className="input mt-1 min-h-16 resize-none text-xs"
                            placeholder="Optional notes"
                          />
                        </label>
                      </div>
                    )}
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <span className={cn('badge', getStatusBadge(spool.status))}>
                          {spool.status}
                        </span>
                        {spool.default_for_material ? (
                          <span className="badge border border-emerald-500/40 bg-emerald-500/15 text-emerald-300">Default</span>
                        ) : (
                          <button onClick={async () => { await updateSpool.mutateAsync({ id: spool.id, data: { ...spool, default_for_material: true } }); showToast({ title: 'Default filament updated', message: `${material?.type?.toUpperCase() || 'Material'} will use this spool by default.`, tone: 'success' }) }} className="text-[11px] text-surface-500 hover:text-emerald-300">Set default</button>
                        )}
                      </div>
                      {spool.location && (
                        <span className="text-xs text-surface-500">
                          📍 {spool.location}
                        </span>
                      )}
                    </div>
                    {spool.notes && editingSpoolId !== spool.id && (
                      <p className="text-xs text-surface-500 line-clamp-2">{spool.notes}</p>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )
      )}

      {/* Catalog Tab */}
      {tab === 'catalog' && (
        materialsLoading ? (
          <div className="text-surface-500">Loading...</div>
        ) : filamentMaterials.length === 0 ? (
          <div className="text-center py-16">
            <Package className="h-16 w-16 mx-auto mb-4 text-surface-600" />
            <h3 className="text-xl font-semibold text-surface-300 mb-2">
              No materials in catalog
            </h3>
            <p className="text-surface-500 mb-4">
              Add materials to your catalog before creating spools
            </p>
            <button
              onClick={() => setShowAddMaterial(true)}
              className="btn btn-primary"
            >
              <Plus className="h-4 w-4 mr-2" />
              Add Material
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            {filamentMaterials.map((material) => (
              <div key={material.id} className="card p-4">
                <div className="flex items-center gap-3 mb-3">
                  <div
                    className="w-10 h-10 rounded-full border-2 border-surface-700"
                    style={{ backgroundColor: material.color_hex || '#666' }}
                  />
                  <div className="flex-1 min-w-0">
                    <h3 className="font-medium text-surface-100 truncate">
                      {material.name}
                    </h3>
                    <p className="text-xs text-surface-500">
                      {material.manufacturer || material.type.toUpperCase()}
                    </p>
                  </div>
                  {editingMaterialId !== material.id && (
                    <button
                      onClick={() => startEditMaterial(material)}
                      className="p-1.5 rounded-lg text-surface-500 hover:text-accent-400 hover:bg-accent-500/10 transition-colors"
                      title="Edit material"
                    >
                      <Edit2 className="h-4 w-4" />
                    </button>
                  )}
                  {confirmDeleteId === material.id ? (
                    <button
                      onClick={() => handleDeleteMaterial(material)}
                      onBlur={() => setConfirmDeleteId(null)}
                      className="text-xs px-2 py-1 rounded bg-red-500/20 text-red-400 border border-red-500/50 hover:bg-red-500/30 transition-colors"
                      autoFocus
                    >
                      Delete?
                    </button>
                  ) : (
                    <button
                      onClick={() => handleDeleteMaterial(material)}
                      className="p-1.5 rounded-lg text-surface-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                      title="Delete material"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                </div>
                {editingMaterialId === material.id ? (
                  <div className="space-y-2">
                    <input value={materialForm.name} onChange={e => setMaterialForm(prev => ({ ...prev, name: e.target.value }))} className="input h-8 text-xs" placeholder="Name" />
                    <input value={materialForm.manufacturer} onChange={e => setMaterialForm(prev => ({ ...prev, manufacturer: e.target.value }))} className="input h-8 text-xs" placeholder="Manufacturer" />
                    <div className="space-y-2 rounded-lg border border-surface-800 bg-surface-900/40 p-2">
                      <div className="flex items-center gap-2">
                        <input
                          type="color"
                          value={materialForm.color_hex && /^#[0-9a-fA-F]{6}$/.test(materialForm.color_hex) ? materialForm.color_hex : '#ffffff'}
                          onChange={e => setMaterialForm(prev => ({ ...prev, color_hex: e.target.value }))}
                          className="h-8 w-10 cursor-pointer rounded border border-surface-700 bg-surface-900 p-0.5"
                          title="Pick color"
                        />
                        <div className="flex-1">
                          <label className="block text-[10px] text-surface-500 mb-1">HEX color</label>
                          <input
                            value={materialForm.color_hex}
                            onChange={e => setMaterialForm(prev => ({ ...prev, color_hex: e.target.value.startsWith('#') ? e.target.value : `#${e.target.value}` }))}
                            className="input h-8 text-xs font-mono uppercase"
                            placeholder="#FFFFFF"
                            maxLength={7}
                          />
                        </div>
                      </div>
                      <input value={materialForm.color} onChange={e => setMaterialForm(prev => ({ ...prev, color: e.target.value }))} className="input h-8 text-xs" placeholder="Color name, ex: Black" />
                    </div>
                    <input value={materialForm.cost_per_kg} onChange={e => setMaterialForm(prev => ({ ...prev, cost_per_kg: e.target.value }))} className="input h-8 text-xs" type="number" min="0" step="0.01" placeholder="Cost/kg" />
                    <div className="flex gap-2 pt-1">
                      <button onClick={() => saveMaterial(material)} className="btn btn-primary text-xs py-1 px-2">Salvar</button>
                      <button onClick={() => setEditingMaterialId(null)} className="btn btn-secondary text-xs py-1 px-2">Cancelar</button>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-1 text-sm">
                    <div className="flex justify-between">
                      <span className="text-surface-500">Type</span>
                      <span className="text-surface-300">{material.type.toUpperCase()}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-surface-500">Color</span>
                      <span className="text-surface-300">{material.color || '—'}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-surface-500">Cost</span>
                      <span className="text-surface-300">${material.cost_per_kg.toFixed(2)}/kg</span>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )
      )}

      {/* Supplies Tab */}
      {tab === 'supplies' && (
        materialsLoading ? (
          <div className="text-surface-500">Loading...</div>
        ) : supplyMaterials.length === 0 ? (
          <div className="text-center py-16">
            <ShoppingCart className="h-16 w-16 mx-auto mb-4 text-surface-600" />
            <h3 className="text-xl font-semibold text-surface-300 mb-2">
              No supplies yet
            </h3>
            <p className="text-surface-500 mb-4">
              Upload Amazon or other receipts to auto-create supply materials
            </p>
          </div>
        ) : (
          <div className="card overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-surface-800">
                  <th className="text-left text-xs font-medium text-surface-500 uppercase px-4 py-2">Item</th>
                  <th className="text-left text-xs font-medium text-surface-500 uppercase px-4 py-2">Vendor</th>
                  <th className="text-right text-xs font-medium text-surface-500 uppercase px-4 py-2">Unit Cost</th>
                  <th className="w-10 px-4 py-2"></th>
                </tr>
              </thead>
              <tbody>
                {supplyMaterials.map((material) => (
                  <tr key={material.id} className="border-b border-surface-800/50">
                    <td className="px-4 py-3 text-surface-200">{material.name}</td>
                    <td className="px-4 py-3 text-surface-400">{material.manufacturer || '—'}</td>
                    <td className="px-4 py-3 text-right text-surface-300">
                      ${material.cost_per_kg.toFixed(2)}
                    </td>
                    <td className="px-4 py-3">
                      {confirmDeleteId === material.id ? (
                        <button
                          onClick={() => handleDeleteMaterial(material)}
                          onBlur={() => setConfirmDeleteId(null)}
                          className="text-xs px-2 py-1 rounded bg-red-500/20 text-red-400 border border-red-500/50 hover:bg-red-500/30 transition-colors"
                          autoFocus
                        >
                          Delete?
                        </button>
                      ) : (
                        <button
                          onClick={() => handleDeleteMaterial(material)}
                          className="p-1 rounded text-surface-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {/* Add Material Modal */}
      {showAddMaterial && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="card w-full max-w-md p-6">
            <h2 className="text-xl font-semibold text-surface-100 mb-4">
              Add Material
            </h2>
            <form onSubmit={handleCreateMaterial}>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    Name *
                  </label>
                  <input
                    type="text"
                    name="name"
                    required
                    className="input"
                    placeholder="Prusament PLA Galaxy Black"
                    autoFocus
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Type *
                    </label>
                    <select name="type" required className="input">
                      <option value="pla">PLA</option>
                      <option value="petg">PETG</option>
                      <option value="abs">ABS</option>
                      <option value="asa">ASA</option>
                      <option value="tpu">TPU</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Manufacturer
                    </label>
                    <input
                      type="text"
                      name="manufacturer"
                      className="input"
                      placeholder="Prusament"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Color
                    </label>
                    <input
                      type="text"
                      name="color"
                      className="input"
                      placeholder="Galaxy Black"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Color Hex
                    </label>
                    <input
                      type="color"
                      name="color_hex"
                      className="input h-10 p-1"
                      defaultValue="#1a1a2e"
                    />
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-surface-300 mb-1">
                    Cost per kg ($)
                  </label>
                  <input
                    type="number"
                    name="cost_per_kg"
                    step="0.01"
                    className="input"
                    placeholder="25.00"
                  />
                </div>
              </div>
              <div className="flex justify-end gap-3 mt-6">
                <button
                  type="button"
                  onClick={() => setShowAddMaterial(false)}
                  className="btn btn-ghost"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createMaterial.isPending}
                  className="btn btn-primary"
                >
                  {createMaterial.isPending ? 'Adding...' : 'Add Material'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add Spool Modal */}
      {showAddSpool && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="card w-full max-w-md p-6">
            <h2 className="text-xl font-semibold text-surface-100 mb-4">
              Add Spool
            </h2>
            {materials.length === 0 ? (
              <div className="text-center py-4">
                <p className="text-surface-500 mb-4">
                  You need to add a material first
                </p>
                <button
                  onClick={() => {
                    setShowAddSpool(false)
                    setShowAddMaterial(true)
                  }}
                  className="btn btn-primary"
                >
                  Add Material
                </button>
              </div>
            ) : (
              <form onSubmit={handleCreateSpool}>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Material *
                    </label>
                    <select name="material_id" required className="input">
                      <option value="">Select material...</option>
                      {materials.map((m) => (
                        <option key={m.id} value={m.id}>
                          {m.name} ({m.type.toUpperCase()})
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Initial Weight (g)
                    </label>
                    <input
                      type="number"
                      name="initial_weight"
                      defaultValue="1000"
                      max="1000"
                      className="input"
                    />
                    <p className="text-xs text-surface-500 mt-1">Cost comes from the selected catalog material.</p>
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-surface-300 mb-1">
                      Location
                    </label>
                    <input
                      type="text"
                      name="location"
                      className="input"
                      placeholder="Dry Box A, Shelf 3"
                    />
                  </div>
                </div>
                <div className="flex justify-end gap-3 mt-6">
                  <button
                    type="button"
                    onClick={() => setShowAddSpool(false)}
                    className="btn btn-ghost"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={createSpool.isPending}
                    className="btn btn-primary"
                  >
                    {createSpool.isPending ? 'Adding...' : 'Add Spool'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

