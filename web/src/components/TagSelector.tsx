import { useEffect, useState } from 'react'
import { Plus, X, Tag as TagIcon } from 'lucide-react'
import { tagsApi } from '../api/client'
import type { Tag } from '../types'

interface TagSelectorProps {
  selectedTags: Tag[]
  onChange: (tags: Tag[]) => void
  entityType: 'part' | 'design'
  entityId?: string
  className?: string
}

export function TagSelector({
  selectedTags,
  onChange,
  entityType,
  entityId,
  className = '',
}: TagSelectorProps) {
  const [allTags, setAllTags] = useState<Tag[]>([])
  const [showDropdown, setShowDropdown] = useState(false)
  const [newTagName, setNewTagName] = useState('')
  const [newTagColor, setNewTagColor] = useState('#6b7280')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    loadTags()
  }, [])

  const loadTags = async () => {
    try {
      const tags = await tagsApi.list()
      setAllTags(tags)
    } catch (err) {
      console.error('Failed to load tags:', err)
    }
  }

  const handleAddTag = async (tag: Tag) => {
    if (selectedTags.find(t => t.id === tag.id)) return

    // If we have an entity ID, persist the change
    if (entityId) {
      try {
        if (entityType === 'part') {
          await tagsApi.addTagToPart(entityId, tag.id)
        } else {
          await tagsApi.addTagToDesign(entityId, tag.id)
        }
      } catch (err) {
        console.error('Failed to add tag:', err)
        return
      }
    }

    onChange([...selectedTags, tag])
    setShowDropdown(false)
  }

  const handleRemoveTag = async (tag: Tag) => {
    // If we have an entity ID, persist the change
    if (entityId) {
      try {
        if (entityType === 'part') {
          await tagsApi.removeTagFromPart(entityId, tag.id)
        } else {
          await tagsApi.removeTagFromDesign(entityId, tag.id)
        }
      } catch (err) {
        console.error('Failed to remove tag:', err)
        return
      }
    }

    onChange(selectedTags.filter(t => t.id !== tag.id))
  }

  const handleCreateTag = async () => {
    if (!newTagName.trim()) return

    setCreating(true)
    try {
      const tag = await tagsApi.create({ name: newTagName.trim(), color: newTagColor })
      setAllTags(prev => [...prev, tag])
      setNewTagName('')
      setNewTagColor('#6b7280')
      // Automatically select the new tag
      handleAddTag(tag)
    } catch (err) {
      console.error('Failed to create tag:', err)
    } finally {
      setCreating(false)
    }
  }

  const availableTags = allTags.filter(t => !selectedTags.find(st => st.id === t.id))

  const colorOptions = [
    '#ef4444', // red
    '#f97316', // orange
    '#eab308', // yellow
    '#22c55e', // green
    '#14b8a6', // teal
    '#3b82f6', // blue
    '#8b5cf6', // violet
    '#ec4899', // pink
    '#6b7280', // gray
  ]

  return (
    <div className={`relative ${className}`}>
      {/* Selected tags */}
      <div className="flex flex-wrap gap-2 min-h-[28px] items-center">
        {selectedTags.map(tag => (
          <span
            key={tag.id}
            className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium text-white"
            style={{ backgroundColor: tag.color }}
          >
            <TagIcon className="w-3 h-3" />
            {tag.name}
            <button
              onClick={() => handleRemoveTag(tag)}
              className="hover:bg-white/20 rounded-full p-0.5"
              title={`Remove tag ${tag.name}`}
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}

        {/* Add tag button */}
        <button
          onClick={() => setShowDropdown(!showDropdown)}
          className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium text-surface-400 border border-dashed border-surface-700 hover:text-surface-100 hover:border-surface-500 transition-colors"
        >
          <Plus className="w-3 h-3" />
          Add tag
        </button>
      </div>

      {/* Dropdown */}
      {showDropdown && (
        <div className="absolute z-10 mt-2 w-64 bg-surface-800 border border-surface-700 rounded-lg shadow-lg">
          {/* Existing tags */}
          {availableTags.length > 0 && (
            <div className="p-2 border-b border-surface-700">
              <div className="text-xs font-medium text-surface-500 mb-2">Existing tags</div>
              <div className="flex flex-wrap gap-1">
                {availableTags.map(tag => (
                  <button
                    key={tag.id}
                    onClick={() => handleAddTag(tag)}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium text-white hover:opacity-80 transition-opacity"
                    style={{ backgroundColor: tag.color }}
                  >
                    <TagIcon className="w-3 h-3" />
                    {tag.name}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Create new tag */}
          <div className="p-2">
            <div className="text-xs font-medium text-surface-500 mb-2">Create new tag</div>
            <div className="flex gap-2 mb-2">
              <input
                type="text"
                value={newTagName}
                onChange={e => setNewTagName(e.target.value)}
                placeholder="Tag name"
                className="input h-auto py-1 flex-1"
                onKeyDown={e => e.key === 'Enter' && handleCreateTag()}
              />
              <button
                onClick={handleCreateTag}
                disabled={!newTagName.trim() || creating}
                className="px-3 py-1 text-sm bg-accent-600 text-white rounded hover:bg-accent-500 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {creating ? '...' : 'Create'}
              </button>
            </div>

            {/* Color picker */}
            <div className="flex gap-1">
              {colorOptions.map(color => (
                <button
                  key={color}
                  onClick={() => setNewTagColor(color)}
                  className={`w-6 h-6 rounded-full border-2 transition-all ${
                    newTagColor === color ? 'border-surface-200 scale-110' : 'border-transparent'
                  }`}
                  style={{ backgroundColor: color }}
                />
              ))}
            </div>
          </div>

          {/* Close button */}
          <div className="p-2 border-t border-surface-700">
            <button
              onClick={() => setShowDropdown(false)}
              className="w-full px-3 py-1.5 text-sm text-surface-400 hover:bg-surface-700 hover:text-surface-100 rounded"
            >
              Close
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

export default TagSelector
