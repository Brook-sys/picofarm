import { useState, useEffect, useCallback } from 'react'
import { Link as RouterLink } from 'react-router-dom'
import { RefreshCw, Package, CheckCircle, Clock, ExternalLink, ShoppingBag, Store, Link, Unlink, Filter } from 'lucide-react'
import { etsyApi, squarespaceApi, projectsApi, salesChannelsApi } from '../api/client'
import type { EtsyReceipt, EtsyListing, SquarespaceOrder, SquarespaceProduct, SyncResult, Project, SalesChannelID, SalesChannelSummary, SalesChannelSyncKind, SalesChannelExternalOrder, SalesChannelExternalProduct } from '../types'
import { cn } from '../lib/utils'
import { formatMoneyCents } from '../lib/currency'
import { useCurrency } from '../hooks/useCurrency'

type Tab = 'orders' | 'products'
type Channel = 'all' | SalesChannelID
type OrderFilter = 'all' | 'unprocessed' | 'processed'

// Unified order type for display
interface UnifiedOrder {
  id: string
  channel: SalesChannelID
  orderNumber: string
  customerName: string
  customerEmail?: string
  totalCents: number
  currency: string
  isProcessed: boolean
  projectId?: string
  createdAt: string
  status?: string
  items: Array<{
    id: string
    name: string
    quantity: number
    priceCents: number
    sku?: string
  }>
  raw: SalesChannelExternalOrder | EtsyReceipt | SquarespaceOrder
}

// Unified product type for display
interface UnifiedProduct {
  id: string
  channel: SalesChannelID
  name: string
  description?: string
  type?: string
  isVisible: boolean
  skus: string[]
  priceCents?: number
  currency?: string
  raw: SalesChannelExternalProduct | EtsyListing | SquarespaceProduct
}

export default function Channels() {
  const { currencyCode } = useCurrency()
  const [tab, setTab] = useState<Tab>('orders')
  const [channel, setChannel] = useState<Channel>('all')
  const [orderFilter, setOrderFilter] = useState<OrderFilter>('all')

  // Provider-neutral channel descriptors and connection status
  const [salesChannels, setSalesChannels] = useState<SalesChannelSummary[]>([])

  // Data
  const [orders, setOrders] = useState<UnifiedOrder[]>([])
  const [products, setProducts] = useState<UnifiedProduct[]>([])
  const [projects, setProjects] = useState<Project[]>([])

  // UI state
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState<Channel | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [syncResult, setSyncResult] = useState<{ channel: string; result: SyncResult } | null>(null)
  const [processingId, setProcessingId] = useState<string | null>(null)
  const [linkingId, setLinkingId] = useState<string | null>(null)
  const [unlinkingId, setUnlinkingId] = useState<string | null>(null)
  const [selectedProject, setSelectedProject] = useState<Record<string, string>>({})

  const visibleSalesChannels = salesChannels
  const hasConfiguredOrPlannedChannel = salesChannels.length > 0
  const syncKind: SalesChannelSyncKind = tab === 'orders' ? 'orders' : 'products'
  const syncableVisibleChannels = visibleSalesChannels.filter(({ descriptor, status }) =>
    status.connected && descriptor.capabilities.includes(tab === 'orders' ? 'orders_read' : 'products_read')
  )

  // Connection status per channel, used to gate the legacy readers below.
  // The provider-neutral read model (the sales_channel_* tables) is only fed
  // by the Shopee / Mercado Livre / OLX adapters — see
  // internal/service/sales_channel_adapters.go. Etsy and Squarespace still
  // live in their own tables (etsy_receipts, squarespace_orders, ...), so a
  // page that read only the neutral model showed nothing at all for the two
  // channels that have real credentials. Both sources are therefore merged:
  // legacy per channel (gated here by that channel's connection status) plus
  // the neutral model (gated by the channel filter).
  const etsyStatus = salesChannels.find(({ descriptor }) => descriptor.id === 'etsy')?.status
  const squarespaceStatus = salesChannels.find(({ descriptor }) => descriptor.id === 'squarespace')?.status

  // Load provider-neutral connection status
  const loadSalesChannelStatus = useCallback(async () => {
    try {
      const response = await salesChannelsApi.list()
      setSalesChannels(response.channels)
    } catch (err) {
      console.error('Failed to load channel status:', err)
    }
  }, [])

  useEffect(() => {
    loadSalesChannelStatus()
  }, [loadSalesChannelStatus])

  // Load orders
  const loadOrders = useCallback(async () => {
    setLoading(true)
    setError(null)
    const unified: UnifiedOrder[] = []

    try {
      const processed = orderFilter === 'all' ? undefined : orderFilter === 'processed'

      // Etsy receipts — legacy storage; the neutral read model has no Etsy rows
      if ((channel === 'all' || channel === 'etsy') && etsyStatus?.connected) {
        try {
          const etsyOrders = await etsyApi.listReceipts({ processed })
          for (const r of etsyOrders) {
            unified.push({
              id: r.id,
              channel: 'etsy',
              orderNumber: String(r.etsy_receipt_id),
              customerName: r.name,
              customerEmail: r.buyer_email,
              totalCents: r.grandtotal_cents,
              currency: r.currency,
              isProcessed: r.is_processed,
              projectId: r.project_id,
              createdAt: r.create_timestamp || r.created_at,
              status: r.status,
              items: (r.items || []).map(i => ({
                id: i.id,
                name: i.title,
                quantity: i.quantity,
                priceCents: i.price_cents,
                sku: i.sku,
              })),
              raw: r,
            })
          }
        } catch (err) {
          console.error('Failed to load Etsy orders:', err)
        }
      }

      // Squarespace orders — same situation: legacy storage only
      if ((channel === 'all' || channel === 'squarespace') && squarespaceStatus?.connected) {
        try {
          const sqOrders = await squarespaceApi.listOrders({ processed })
          for (const o of sqOrders) {
            unified.push({
              id: o.id,
              channel: 'squarespace',
              orderNumber: o.order_number,
              customerName: o.customer_name || o.customer_email,
              customerEmail: o.customer_email,
              totalCents: o.grand_total_cents,
              currency: o.currency,
              isProcessed: o.is_processed,
              projectId: o.project_id,
              createdAt: o.created_on || o.created_at,
              status: o.fulfillment_status,
              items: (o.items || []).map(i => ({
                id: i.id,
                name: i.product_name,
                quantity: i.quantity,
                priceCents: i.unit_price_cents,
                sku: i.sku,
              })),
              raw: o,
            })
          }
        } catch (err) {
          console.error('Failed to load Squarespace orders:', err)
        }
      }

      // Provider-neutral read model — today only Shopee / Mercado Livre / OLX
      const response = await salesChannelsApi.listOrders({
        channel: channel === 'all' ? undefined : channel,
        processed,
      })
      for (const order of response.orders) {
        unified.push({
          id: order.id,
          channel: order.channel,
          orderNumber: order.order_number || order.external_order_id,
          customerName: order.customer_name || order.customer_email || 'Unknown customer',
          customerEmail: order.customer_email,
          totalCents: order.total_cents,
          currency: order.currency,
          isProcessed: order.is_processed,
          projectId: order.order_id,
          createdAt: order.created_at,
          status: order.status,
          items: (order.items || []).map(item => ({
            id: item.id,
            name: item.title,
            quantity: item.quantity,
            priceCents: item.unit_price_cents,
            sku: item.sku,
          })),
          raw: order,
        })
      }

      // Sort by date descending
      unified.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
      setOrders(unified)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load orders')
    } finally {
      setLoading(false)
    }
  }, [channel, orderFilter, etsyStatus?.connected, squarespaceStatus?.connected])

  // Load products
  const loadProducts = useCallback(async () => {
    setLoading(true)
    setError(null)
    const unified: UnifiedProduct[] = []

    try {
      // Load projects for linking. Products link to projects: the recipe /
      // template catalog was removed from the product (projects and tasks are
      // the catalog), and /api/templates no longer exists, so requesting it
      // here 404'd and took the whole product list down with it.
      const projectList = await projectsApi.list()
      setProjects(projectList)

      // Etsy listings — legacy storage; the neutral read model has no Etsy rows
      if ((channel === 'all' || channel === 'etsy') && etsyStatus?.connected) {
        try {
          const listings = await etsyApi.listListings()
          for (const l of listings) {
            unified.push({
              id: l.id,
              channel: 'etsy',
              name: l.title,
              description: l.description,
              type: l.state,
              isVisible: l.state === 'active',
              skus: l.skus || [],
              priceCents: l.price_cents,
              currency: l.currency,
              raw: l,
            })
          }
        } catch (err) {
          console.error('Failed to load Etsy listings:', err)
        }
      }

      // Squarespace products — same situation: legacy storage only
      if ((channel === 'all' || channel === 'squarespace') && squarespaceStatus?.connected) {
        try {
          const prods = await squarespaceApi.listProducts()
          for (const p of prods) {
            unified.push({
              id: p.id,
              channel: 'squarespace',
              name: p.name,
              description: p.description,
              type: p.type,
              isVisible: p.is_visible,
              skus: (p.variants || []).map(v => v.sku).filter(Boolean) as string[],
              priceCents: p.variants?.[0]?.price_cents,
              currency: p.variants?.[0]?.currency,
              raw: p,
            })
          }
        } catch (err) {
          console.error('Failed to load Squarespace products:', err)
        }
      }

      // Provider-neutral read model — today only Shopee / Mercado Livre / OLX
      const response = await salesChannelsApi.listProducts({
        channel: channel === 'all' ? undefined : channel,
      })
      for (const product of response.products) {
        unified.push({
          id: product.id,
          channel: product.channel,
          name: product.title,
          description: product.description,
          type: product.status,
          isVisible: product.is_visible,
          skus: (product.variants || []).map(v => v.sku).filter(Boolean) as string[],
          priceCents: product.price_cents || product.variants?.[0]?.price_cents,
          currency: product.currency || product.variants?.[0]?.currency,
          raw: product,
        })
      }

      // Sort by name
      unified.sort((a, b) => a.name.localeCompare(b.name))
      setProducts(unified)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load products')
    } finally {
      setLoading(false)
    }
  }, [channel, etsyStatus?.connected, squarespaceStatus?.connected])

  // Load data when tab or filters change
  useEffect(() => {
    if (tab === 'orders') {
      loadOrders()
    } else {
      loadProducts()
    }
  }, [tab, loadOrders, loadProducts])

  // Sync handlers
  async function handleSync(syncChannel: Channel) {
    if (syncChannel === 'all') return
    setSyncing(syncChannel)
    setError(null)
    setSyncResult(null)

    try {
      const selectedChannel = salesChannels.find(({ descriptor }) => descriptor.id === syncChannel)
      if (!selectedChannel?.descriptor.capabilities.includes(tab === 'orders' ? 'orders_read' : 'products_read')) {
        throw new Error(`${selectedChannel?.descriptor.display_name || syncChannel} does not support ${tab} sync`)
      }
      const { result } = await salesChannelsApi.sync(syncChannel, syncKind)
      setSyncResult({ channel: syncChannel, result })
      await loadSalesChannelStatus()
      if (tab === 'orders') {
        await loadOrders()
      } else {
        await loadProducts()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : `Failed to sync ${syncChannel}`)
    } finally {
      setSyncing(null)
    }
  }

  async function handleSyncAll() {
    setSyncing('all')
    setError(null)
    setSyncResult(null)

    try {
      const results: SyncResult = { total_fetched: 0, created: 0, updated: 0, skipped: 0, errors: 0 }

      for (const { descriptor } of syncableVisibleChannels) {
        const { result: r } = await salesChannelsApi.sync(descriptor.id, syncKind)
        results.total_fetched += r.total_fetched
        results.created += r.created
        results.updated += r.updated
        results.skipped += r.skipped
        results.errors += r.errors
      }

      setSyncResult({ channel: 'all', result: results })
      await loadSalesChannelStatus()
      if (tab === 'orders') {
        await loadOrders()
      } else {
        await loadProducts()
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync')
    } finally {
      setSyncing(null)
    }
  }

  // Process order
  async function handleProcess(order: UnifiedOrder) {
    setProcessingId(order.id)
    setError(null)
    try {
      if (order.channel === 'etsy') {
        await etsyApi.processReceipt(order.id)
      } else if (order.channel === 'squarespace') {
        await squarespaceApi.processOrder(order.id)
      } else {
        await salesChannelsApi.processOrder(order.id)
      }
      await loadOrders()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to process order')
    } finally {
      setProcessingId(null)
    }
  }

  // Link product to project
  async function handleLink(product: UnifiedProduct) {
    const projectId = selectedProject[product.id]
    if (!projectId) {
      setError('Please select a project')
      return
    }

    setLinkingId(product.id)
    setError(null)
    try {
      const sku = product.skus[0]
      if (product.channel === 'etsy') {
        await etsyApi.linkListing(product.id, { project_id: projectId, sku })
      } else if (product.channel === 'squarespace') {
        await squarespaceApi.linkProduct(product.id, projectId, sku)
      } else {
        await salesChannelsApi.linkProduct(product.id, { project_id: projectId, sku })
      }
      await loadProducts()
      setSelectedProject(prev => ({ ...prev, [product.id]: '' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to link product')
    } finally {
      setLinkingId(null)
    }
  }

  // Unlink product from project — the inverse of handleLink. The link is
  // stored per (product, project), so the same project selection identifies
  // which link to remove; the read model does not expose the current link.
  async function handleUnlink(product: UnifiedProduct) {
    const projectId = selectedProject[product.id]
    if (!projectId) {
      setError('Please select the linked project')
      return
    }

    setUnlinkingId(product.id)
    setError(null)
    try {
      if (product.channel === 'etsy') {
        await etsyApi.unlinkListing(product.id, projectId)
      } else if (product.channel === 'squarespace') {
        await squarespaceApi.unlinkProduct(product.id, projectId)
      } else {
        await salesChannelsApi.unlinkProduct(product.id, { project_id: projectId })
      }
      await loadProducts()
      setSelectedProject(prev => ({ ...prev, [product.id]: '' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to unlink product')
    } finally {
      setUnlinkingId(null)
    }
  }

  function formatCents(cents: number, currency: string = currencyCode) {
    return formatMoneyCents(cents, currency)
  }

  function formatDate(dateStr: string) {
    return new Date(dateStr).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  }

  function channelTone(channelID: SalesChannelID) {
    switch (channelID) {
      case 'etsy':
        return 'bg-orange-500/10 border-orange-500/30 text-orange-300'
      case 'squarespace':
        return 'bg-purple-500/10 border-purple-500/30 text-purple-300'
      case 'mercado_livre':
        return 'bg-yellow-500/10 border-yellow-500/30 text-yellow-300'
      case 'shopee':
        return 'bg-rose-500/10 border-rose-500/30 text-rose-300'
      case 'olx':
        return 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
      default:
        return 'bg-sky-500/10 border-sky-500/30 text-sky-300'
    }
  }

  function channelIcon(channelID: SalesChannelID, className = 'h-3.5 w-3.5') {
    return channelID === 'etsy' ? <Store className={className} /> : <ShoppingBag className={className} />
  }

  function syncButtonTone(channelID: Channel) {
    switch (channelID) {
      case 'etsy':
        return 'bg-orange-500 hover:bg-orange-600'
      case 'squarespace':
        return 'bg-purple-500 hover:bg-purple-600'
      case 'mercado_livre':
        return 'bg-yellow-600 hover:bg-yellow-500'
      case 'shopee':
        return 'bg-rose-600 hover:bg-rose-500'
      case 'olx':
        return 'bg-emerald-600 hover:bg-emerald-500'
      default:
        return 'bg-accent-600 hover:bg-accent-500'
    }
  }

  return (
    <div className="p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-display font-semibold text-surface-100">
            Sales Channels
          </h1>
          <p className="mt-1 text-sm text-surface-400">
            Manage orders and products from connected marketplaces
          </p>
        </div>

        {/* Connection Status Pills */}
        <div className="flex items-center gap-2">
          {visibleSalesChannels.map(({ descriptor, status }) => (
            <div
              key={descriptor.id}
              className={cn(
                'flex items-center gap-1.5 px-2 py-1 rounded-lg border',
                channelTone(descriptor.id),
                !status.connected && 'opacity-70'
              )}
              title={status.connected ? 'Connected' : status.last_error || 'Not connected yet'}
            >
              {channelIcon(descriptor.id)}
              <span className="text-xs">{status.display_name || descriptor.display_name}</span>
              {!status.connected && <span className="text-[10px] uppercase tracking-wide opacity-70">planned</span>}
            </div>
          ))}
          {!hasConfiguredOrPlannedChannel && (
            <RouterLink
              to="/settings"
              className="text-sm text-accent-400 hover:text-accent-300"
            >
              Connect a channel in Settings
            </RouterLink>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-4 mb-6 border-b border-surface-800">
        <button
          onClick={() => setTab('orders')}
          className={cn(
            'pb-3 text-sm font-medium border-b-2 transition-colors',
            tab === 'orders'
              ? 'border-accent-500 text-accent-400'
              : 'border-transparent text-surface-400 hover:text-surface-200'
          )}
        >
          Orders
        </button>
        <button
          onClick={() => setTab('products')}
          className={cn(
            'pb-3 text-sm font-medium border-b-2 transition-colors',
            tab === 'products'
              ? 'border-accent-500 text-accent-400'
              : 'border-transparent text-surface-400 hover:text-surface-200'
          )}
        >
          Products / Listings
        </button>
      </div>

      {/* Filters and Actions */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          {/* Channel Filter */}
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-surface-500" />
            <select
              value={channel}
              onChange={(e) => setChannel(e.target.value as Channel)}
              className="input h-auto py-1.5 w-auto"
            >
              <option value="all">All Channels</option>
              {visibleSalesChannels.map(({ descriptor }) => (
                <option key={descriptor.id} value={descriptor.id}>{descriptor.display_name}</option>
              ))}
            </select>
          </div>

          {/* Order Status Filter (orders tab only) */}
          {tab === 'orders' && (
            <div className="flex gap-1">
              {(['all', 'unprocessed', 'processed'] as const).map((f) => (
                <button
                  key={f}
                  onClick={() => setOrderFilter(f)}
                  className={cn(
                    'px-3 py-1.5 text-xs rounded-lg transition-colors',
                    orderFilter === f
                      ? 'bg-accent-600 text-white'
                      : 'bg-surface-800 text-surface-300 hover:bg-surface-700'
                  )}
                >
                  {f.charAt(0).toUpperCase() + f.slice(1)}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Sync Buttons */}
        <div className="flex items-center gap-2">
          {channel === 'all' ? (
            <button
              onClick={handleSyncAll}
              disabled={syncing !== null || syncableVisibleChannels.length === 0}
              className="flex items-center gap-2 px-4 py-2 bg-accent-600 text-white rounded-lg hover:bg-accent-500 disabled:opacity-50 text-sm"
            >
              <RefreshCw className={cn('h-4 w-4', syncing && 'animate-spin')} />
              {syncing ? 'Syncing...' : 'Sync All'}
            </button>
          ) : (
            <button
              onClick={() => handleSync(channel)}
              disabled={syncing !== null || !syncableVisibleChannels.some(({ descriptor }) => descriptor.id === channel)}
              className={cn(
                'flex items-center gap-2 px-4 py-2 text-white rounded-lg disabled:opacity-50 text-sm',
                syncButtonTone(channel)
              )}
            >
              <RefreshCw className={cn('h-4 w-4', syncing === channel && 'animate-spin')} />
              {syncing === channel ? 'Syncing...' : `Sync ${visibleSalesChannels.find(({ descriptor }) => descriptor.id === channel)?.descriptor.display_name || channel}`}
            </button>
          )}
        </div>
      </div>

      {/* Sync Result */}
      {syncResult && (
        <div className="mb-6 p-4 bg-green-500/10 border border-green-500/30 rounded-lg">
          <p className="text-green-400">
            Synced {syncResult.result.total_fetched} {tab}: {syncResult.result.created} new, {syncResult.result.updated} updated
            {syncResult.result.errors > 0 && `, ${syncResult.result.errors} errors`}
          </p>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="mb-6 p-4 bg-red-500/10 border border-red-500/30 rounded-lg">
          <p className="text-red-400">{error}</p>
        </div>
      )}

      {/* Content */}
      {!hasConfiguredOrPlannedChannel ? (
        <div className="text-center py-12">
          <Package className="h-12 w-12 mx-auto text-surface-600 mb-3" />
          <p className="text-surface-400">No sales channels connected</p>
          <RouterLink
            to="/settings"
            className="mt-4 inline-block text-accent-400 hover:text-accent-300"
          >
            Connect or plan a sales channel in Settings
          </RouterLink>
        </div>
      ) : loading ? (
        <div className="text-center py-12 text-surface-400">Loading...</div>
      ) : tab === 'orders' ? (
        /* Orders List */
        orders.length === 0 ? (
          <div className="text-center py-12">
            <Package className="h-12 w-12 mx-auto text-surface-600 mb-3" />
            <p className="text-surface-400">No orders found</p>
          </div>
        ) : (
          <div className="space-y-4">
            {orders.map((order) => (
              <div
                key={`${order.channel}-${order.id}`}
                className="bg-surface-900 border border-surface-800 rounded-lg p-4"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      {/* Channel Icon */}
                      {channelIcon(order.channel, 'h-4 w-4')}
                      <h3 className="text-lg font-medium text-surface-100">
                        {order.customerName}
                      </h3>
                      <span className="text-sm text-surface-500">
                        #{order.orderNumber}
                      </span>
                      {order.isProcessed ? (
                        <span className="flex items-center gap-1 text-xs px-2 py-0.5 bg-green-500/20 text-green-400 rounded">
                          <CheckCircle className="h-3 w-3" />
                          Processed
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-xs px-2 py-0.5 bg-yellow-500/20 text-yellow-400 rounded">
                          <Clock className="h-3 w-3" />
                          Pending
                        </span>
                      )}
                      {order.status && (
                        <span className="text-xs px-2 py-0.5 bg-surface-700 text-surface-300 rounded">
                          {order.status}
                        </span>
                      )}
                    </div>

                    <div className="mt-2 flex items-center gap-4 text-sm text-surface-400">
                      <span>{formatDate(order.createdAt)}</span>
                      <span className="font-medium text-surface-200">
                        {formatCents(order.totalCents, order.currency)}
                      </span>
                    </div>

                    {/* Items */}
                    {order.items.length > 0 && (
                      <div className="mt-3 space-y-1">
                        {order.items.map((item) => (
                          <div key={item.id} className="flex items-center justify-between text-sm">
                            <div className="flex items-center gap-2">
                              <span className="text-surface-300">{item.name}</span>
                              {item.quantity > 1 && (
                                <span className="text-surface-500">x{item.quantity}</span>
                              )}
                              {item.sku && (
                                <span className="text-xs text-surface-600 font-mono">
                                  SKU: {item.sku}
                                </span>
                              )}
                            </div>
                            <span className="text-surface-400">
                              {formatCents(item.priceCents * item.quantity, order.currency)}
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Actions */}
                  <div className="flex items-center gap-2 ml-4">
                    {!order.isProcessed && (
                      <button
                        onClick={() => handleProcess(order)}
                        disabled={processingId === order.id}
                        className="px-3 py-1.5 text-sm bg-accent-600 text-white rounded hover:bg-accent-500 disabled:opacity-50"
                      >
                        {processingId === order.id ? 'Processing...' : 'Create Project'}
                      </button>
                    )}
                    {order.projectId && (
                      <a
                        href={`/projects/${order.projectId}`}
                        className="px-3 py-1.5 text-sm bg-surface-700 text-surface-200 rounded hover:bg-surface-600 flex items-center gap-1"
                      >
                        View Project
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )
      ) : (
        /* Products List */
        products.length === 0 ? (
          <div className="text-center py-12">
            <Package className="h-12 w-12 mx-auto text-surface-600 mb-3" />
            <p className="text-surface-400">No products found</p>
          </div>
        ) : (
          <div className="space-y-4">
            {products.map((product) => (
              <div
                key={`${product.channel}-${product.id}`}
                className="bg-surface-900 border border-surface-800 rounded-lg p-4"
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3">
                      {/* Channel Icon */}
                      {channelIcon(product.channel, 'h-4 w-4')}
                      <h3 className="text-lg font-medium text-surface-100">
                        {product.name}
                      </h3>
                      {product.type && (
                        <span className="text-xs px-2 py-0.5 bg-surface-700 text-surface-300 rounded">
                          {product.type}
                        </span>
                      )}
                      {!product.isVisible && (
                        <span className="text-xs px-2 py-0.5 bg-yellow-500/20 text-yellow-400 rounded">
                          Hidden
                        </span>
                      )}
                    </div>

                    {product.description && (
                      <p className="mt-1 text-sm text-surface-500 line-clamp-1">
                        {product.description}
                      </p>
                    )}

                    <div className="mt-2 flex items-center gap-4 text-sm text-surface-400">
                      {product.priceCents !== undefined && (
                        <span className="font-medium text-surface-200">
                          {formatCents(product.priceCents, product.currency)}
                        </span>
                      )}
                      {product.skus.length > 0 && (
                        <span className="font-mono text-xs">
                          SKU: {product.skus.slice(0, 3).join(', ')}{product.skus.length > 3 && '...'}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Link to Project */}
                  <div className="flex items-center gap-2 ml-4">
                    <select
                      value={selectedProject[product.id] || ''}
                      onChange={(e) => setSelectedProject(prev => ({ ...prev, [product.id]: e.target.value }))}
                      className="input h-auto py-1 w-auto"
                    >
                      <option value="">Select project...</option>
                      {projects.map((project) => (
                        <option key={project.id} value={project.id}>
                          {project.name} {project.sku && `(${project.sku})`}
                        </option>
                      ))}
                    </select>
                    <button
                      onClick={() => handleLink(product)}
                      disabled={linkingId === product.id || !selectedProject[product.id]}
                      className="flex items-center gap-1 px-3 py-1.5 text-sm bg-accent-600 text-white rounded hover:bg-accent-500 disabled:opacity-50"
                    >
                      {linkingId === product.id ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <Link className="h-3 w-3" />
                      )}
                      Link
                    </button>
                    <button
                      onClick={() => handleUnlink(product)}
                      disabled={unlinkingId === product.id || !selectedProject[product.id]}
                      title="Remove the link between this product and the selected project"
                      className="flex items-center gap-1 px-3 py-1.5 text-sm border border-surface-600 text-surface-300 rounded hover:bg-surface-800 disabled:opacity-50"
                    >
                      {unlinkingId === product.id ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <Unlink className="h-3 w-3" />
                      )}
                      Unlink
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )
      )}
    </div>
  )
}
