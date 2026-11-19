import { useTranslation } from 'react-i18next'
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Package, Plus, Clock, CheckCircle, Truck, XCircle, ChevronRight, RefreshCw } from 'lucide-react'
import { ordersApi } from '../api/client'
import type { Order, OrderStatus, OrderSource } from '../types'

const statusConfig: Record<OrderStatus, { label: string; color: string; icon: React.ReactNode }> = {
  pending: { label: 'Pending', color: 'bg-gray-100 text-gray-700', icon: <Clock className="w-4 h-4" /> },
  in_progress: { label: 'In Progress', color: 'bg-blue-100 text-blue-700', icon: <RefreshCw className="w-4 h-4" /> },
  completed: { label: 'Completed', color: 'bg-green-100 text-green-700', icon: <CheckCircle className="w-4 h-4" /> },
  shipped: { label: 'Shipped', color: 'bg-purple-100 text-purple-700', icon: <Truck className="w-4 h-4" /> },
  cancelled: { label: 'Cancelled', color: 'bg-red-100 text-red-700', icon: <XCircle className="w-4 h-4" /> },
}

const sourceLabels: Record<OrderSource, string> = {
  manual: 'Manual',
  etsy: 'Etsy',
  squarespace: 'Squarespace',
  shopify: 'Shopify',
  quote: 'Quote',
}

export function Orders() {
  const { t, i18n } = useTranslation('commerce')
  const locale = i18n.resolvedLanguage?.startsWith('en') ? 'en-US' : 'pt-BR'

  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState<OrderStatus | ''>('')
  const [sourceFilter, setSourceFilter] = useState<OrderSource | ''>('')
  const [showCreateModal, setShowCreateModal] = useState(false)

  const loadOrders = useCallback(async () => {
    setLoading(true)
    try {
      const params: { status?: string; source?: string } = {}
      if (statusFilter) params.status = statusFilter
      if (sourceFilter) params.source = sourceFilter
      const data = await ordersApi.list(params)
      setOrders(data)
    } catch (err) {
      console.error('Failed to load orders:', err)
    } finally {
      setLoading(false)
    }
  }, [sourceFilter, statusFilter])

  useEffect(() => {
    loadOrders()
  }, [loadOrders])

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '-'
    return new Date(dateStr).toLocaleDateString(locale)
  }

  const getDueDateClass = (order: Order) => {
    if (!order.due_date || order.status === 'completed' || order.status === 'shipped') return ''
    const dueDate = new Date(order.due_date)
    const now = new Date()
    const hoursUntilDue = (dueDate.getTime() - now.getTime()) / (1000 * 60 * 60)
    if (hoursUntilDue < 0) return 'text-red-600 font-semibold'
    if (hoursUntilDue < 24) return 'text-red-500'
    if (hoursUntilDue < 72) return 'text-amber-500'
    return ''
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">{t("Orders")}</h1>
          <p className="text-gray-600">{t("Manage customer orders from all sources")}</p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
        >
          <Plus className="w-4 h-4" />

          {t("New Order")}
        </button>
      </div>

      {/* Filters */}
      <div className="flex gap-4 mb-6">
        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value as OrderStatus | '')}
          className="input h-auto py-2 w-auto"
        >
          <option value="">{t("All Statuses")}</option>
          {Object.entries(statusConfig).map(([status, config]) => (
            <option key={status} value={status}>
              {t(config.label)}
            </option>
          ))}
        </select>

        <select
          value={sourceFilter}
          onChange={e => setSourceFilter(e.target.value as OrderSource | '')}
          className="input h-auto py-2 w-auto"
        >
          <option value="">{t("All Sources")}</option>
          {Object.entries(sourceLabels).map(([source, label]) => (
            <option key={source} value={source}>
              {t(label)}
            </option>
          ))}
        </select>
      </div>

      {/* Orders list */}
      {loading ? (
        <div className="text-center py-12 text-gray-500">{t("Loading orders...")}</div>
      ) : orders.length === 0 ? (
        <div className="text-center py-12">
          <Package className="w-12 h-12 mx-auto text-gray-400 mb-4" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">{t("No orders yet")}</h3>
          <p className="text-gray-500 mb-4">{t("Create a manual order or sync from a marketplace")}</p>
          <button
            onClick={() => setShowCreateModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            <Plus className="w-4 h-4" />

            {t("Create Order")}
          </button>
        </div>
      ) : (
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">

                  {t("Customer")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">

                  {t("Source")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">

                  {t("Status")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">

                  {t("Due Date")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">

                  {t("Items")}
                </th>
                <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">

                  {t("Created")}
                </th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {orders.map(order => (
                <tr key={order.id} className="hover:bg-gray-50">
                  <td className="px-4 py-4">
                    <div className="font-medium text-gray-900">{order.customer_name}</div>
                    {order.customer_email && (
                      <div className="text-sm text-gray-500">{order.customer_email}</div>
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <span className="text-sm text-gray-600">
                      {t(sourceLabels[order.source])}
                    </span>
                    {order.source_order_id && (
                      <div className="text-xs text-gray-400">#{order.source_order_id}</div>
                    )}
                  </td>
                  <td className="px-4 py-4">
                    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${statusConfig[order.status].color}`}>
                      {statusConfig[order.status].icon}
                      {t(statusConfig[order.status].label)}
                    </span>
                  </td>
                  <td className={`px-4 py-4 text-sm ${getDueDateClass(order)}`}>
                    {formatDate(order.due_date)}
                  </td>
                  <td className="px-4 py-4 text-sm text-gray-600">
                    {order.items?.length || 0}  {t("items")}
                  </td>
                  <td className="px-4 py-4 text-sm text-gray-500">
                    {formatDate(order.created_at)}
                  </td>
                  <td className="px-4 py-4">
                    <Link aria-label={t("Open details")}
                      to={`/orders/${order.id}`}
                      className="text-blue-600 hover:text-blue-700"
                    >
                      <ChevronRight className="w-5 h-5" />
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create Order Modal */}
      {showCreateModal && (
        <CreateOrderModal
          onClose={() => setShowCreateModal(false)}
          onCreated={order => {
            setOrders(prev => [order, ...prev])
            setShowCreateModal(false)
          }}
        />
      )}
    </div>
  )
}

interface CreateOrderModalProps {
  onClose: () => void
  onCreated: (order: Order) => void
}

function CreateOrderModal({ onClose, onCreated }: CreateOrderModalProps) {
  const { t } = useTranslation('commerce')

  const [customerName, setCustomerName] = useState('')
  const [customerEmail, setCustomerEmail] = useState('')
  const [dueDate, setDueDate] = useState('')
  const [notes, setNotes] = useState('')
  const [priority, setPriority] = useState(0)
  const [creating, setCreating] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!customerName.trim()) return

    setCreating(true)
    try {
      const order = await ordersApi.create({
        customer_name: customerName.trim(),
        customer_email: customerEmail.trim() || undefined,
        due_date: dueDate || undefined,
        notes: notes.trim() || undefined,
        priority,
      })
      onCreated(order)
    } catch (err) {
      console.error('Failed to create order:', err)
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md p-6">
        <h2 className="text-xl font-bold mb-4">{t("Create New Order")}</h2>

        <form onSubmit={handleSubmit}>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-surface-300 mb-1">

                {t("Customer Name *")}
              </label>
              <input
                type="text"
                value={customerName}
                onChange={e => setCustomerName(e.target.value)}
                className="input"
                placeholder={t("Customer name")}
                required
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-surface-300 mb-1">

                {t("Customer Email")}
              </label>
              <input
                type="email"
                value={customerEmail}
                onChange={e => setCustomerEmail(e.target.value)}
                className="input"
                placeholder={t("Customer email")}
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-surface-300 mb-1">

                {t("Due Date")}
              </label>
              <input
                type="datetime-local"
                value={dueDate}
                onChange={e => setDueDate(e.target.value)}
                className="input"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-surface-300 mb-1">

                {t("Priority")}
              </label>
              <select
                value={priority}
                onChange={e => setPriority(Number(e.target.value))}
                className="input"
              >
                <option value={0}>{t("Normal")}</option>
                <option value={1}>{t("High")}</option>
                <option value={2}>{t("Urgent")}</option>
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-surface-300 mb-1">

                {t("Notes")}
              </label>
              <textarea
                value={notes}
                onChange={e => setNotes(e.target.value)}
                className="input"
                rows={3}
                placeholder={t("Optional notes...")}
              />
            </div>
          </div>

          <div className="flex justify-end gap-3 mt-6">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-gray-600 hover:text-gray-800"
            >

              {t("Cancel")}
            </button>
            <button
              type="submit"
              disabled={!customerName.trim() || creating}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {creating ? t("Creating...") : t("Create Order")}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export default Orders
