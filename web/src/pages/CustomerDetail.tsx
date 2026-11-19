import { useTranslation } from 'react-i18next'
import { useCallback, useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { ArrowLeft, Mail, Building2, Phone, FileText, Package, Trash2, Plus, MapPin } from 'lucide-react'
import { customersApi, quotesApi, ordersApi } from '../api/client'
import type { Customer, Quote, Order, Address } from '../types'

const displayLabels: Record<string, string> = {
  marketplace: 'Marketplace', etsy: 'Etsy', website: 'Website', direct: 'Direct', other: 'Other',
  manual: 'Manual', imported: 'Imported',
  pending: 'Pending', in_progress: 'In Progress', completed: 'Completed', shipped: 'Shipped', cancelled: 'Cancelled',
  draft: 'Draft', sent: 'Sent', accepted: 'Accepted', rejected: 'Rejected', expired: 'Expired',
  confirmed: 'Confirmed', processing: 'Processing...', failed: 'Parse Failed', pending_review: 'Pending Review',
  queued: 'Queued', printing: 'Printing', paused: 'Paused',
}

export default function CustomerDetail() {
  const { t, i18n } = useTranslation('commerce')
  const locale = i18n.resolvedLanguage?.startsWith('en') ? 'en-US' : 'pt-BR'

  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [quotes, setQuotes] = useState<Quote[]>([])
  const [orders, setOrders] = useState<Order[]>([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState<'quotes' | 'orders'>('quotes')
  const [editing, setEditing] = useState(false)
  const [showNewQuote, setShowNewQuote] = useState(false)
  const [billingAddr, setBillingAddr] = useState<Address>({})
  const [shippingAddr, setShippingAddr] = useState<Address>({})

  const loadData = useCallback(async () => {
    if (!id) return
    try {
      const customerData = await customersApi.get(id)
      setCustomer(customerData)

      // Load related data separately so failures don't block the page
      quotesApi.list({ customer_id: id }).then(setQuotes).catch(() => {})
      ordersApi.list().then(data => setOrders(data.filter(o => o.customer_id === id))).catch(() => {})
    } catch (err) {
      console.error('Failed to load customer:', err)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleDelete = async () => {
    if (!id || !confirm(t("Delete this customer? This cannot be undone."))) return
    try {
      await customersApi.delete(id)
      navigate('/customers')
    } catch (err) {
      console.error('Failed to delete customer:', err)
    }
  }

  const handleCreateQuote = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!id) return
    const form = e.currentTarget
    const formData = new FormData(form)
    try {
      const quote = await quotesApi.create({
        customer_id: id,
        title: formData.get('title') as string,
        notes: formData.get('notes') as string || undefined,
      })
      navigate(`/quotes/${quote.id}`)
    } catch (err) {
      console.error('Failed to create quote:', err)
    }
  }

  const startEditing = () => {
    if (customer) {
      setBillingAddr(customer.billing_address || {})
      setShippingAddr(customer.shipping_address || {})
    }
    setEditing(true)
  }

  const handleUpdate = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!id) return
    const form = e.currentTarget
    const formData = new FormData(form)
    const hasAddr = (a: Address) => !!(a.line1 || a.city || a.state || a.zip)
    try {
      const updated = await customersApi.update(id, {
        name: formData.get('name') as string,
        email: formData.get('email') as string || undefined,
        company: formData.get('company') as string || undefined,
        phone: formData.get('phone') as string || undefined,
        notes: formData.get('notes') as string || undefined,
        billing_address: hasAddr(billingAddr) ? billingAddr : undefined,
        shipping_address: hasAddr(shippingAddr) ? shippingAddr : undefined,
      })
      setCustomer(updated)
      setEditing(false)
    } catch (err) {
      console.error('Failed to update customer:', err)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-accent-500" />
      </div>
    )
  }

  if (!customer) {
    return (
      <div className="p-6 text-center text-surface-400">{t("Customer not found")}</div>
    )
  }

  const statusColors: Record<string, string> = {
    draft: 'bg-surface-700 text-surface-300',
    sent: 'bg-blue-500/20 text-blue-400',
    accepted: 'bg-green-500/20 text-green-400',
    rejected: 'bg-red-500/20 text-red-400',
    expired: 'bg-surface-700 text-surface-400',
  }

  return (
    <div className="p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <Link to="/customers" className="inline-flex items-center gap-1.5 text-sm text-surface-400 hover:text-surface-200 mb-4">
          <ArrowLeft className="h-4 w-4" />

          {t("Back to Customers")}
        </Link>
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-display font-bold text-surface-100">{customer.name}</h1>
          <div className="flex items-center gap-2">
            <button
              onClick={() => editing ? setEditing(false) : startEditing()}
              className="px-4 py-2 text-sm text-surface-400 hover:text-surface-200 border border-surface-700 rounded-lg hover:bg-surface-800"
            >
              {editing ? t("Cancel") : t("Edit")}
            </button>
            <button
              onClick={handleDelete}
              className="inline-flex items-center gap-1.5 px-4 py-2 text-sm text-red-400 hover:bg-red-500/10 border border-red-500/30 rounded-lg"
              title={t("Delete customer")}
            >
              <Trash2 className="h-4 w-4" />

              {t("Delete")}
            </button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Info / Edit */}
        <div className="lg:col-span-1">
          {editing ? (
            <div className="bg-surface-900 border border-surface-800 rounded-lg p-4">
              <form onSubmit={handleUpdate} className="space-y-3">
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Name")}</label>
                  <input name="name" defaultValue={customer.name} required className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Email")}</label>
                  <input name="email" type="email" defaultValue={customer.email} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Company")}</label>
                  <input name="company" defaultValue={customer.company} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Phone")}</label>
                  <input name="phone" defaultValue={customer.phone} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Notes")}</label>
                  <textarea name="notes" defaultValue={customer.notes} rows={3} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                </div>

                {/* Billing Address */}
                <div className="pt-2 border-t border-surface-800">
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Billing Address")}</label>
                  <div className="space-y-1.5">
                    <input value={billingAddr.line1 || ''} onChange={e => setBillingAddr(a => ({ ...a, line1: e.target.value }))} placeholder={t("Address line 1")} className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                    <input value={billingAddr.line2 || ''} onChange={e => setBillingAddr(a => ({ ...a, line2: e.target.value }))} placeholder={t("Address line 2")} className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                    <div className="grid grid-cols-3 gap-1.5">
                      <input value={billingAddr.city || ''} onChange={e => setBillingAddr(a => ({ ...a, city: e.target.value }))} placeholder={t("City")} className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                      <input value={billingAddr.state || ''} onChange={e => setBillingAddr(a => ({ ...a, state: e.target.value }))} placeholder={t("State")} className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                      <input value={billingAddr.zip || ''} onChange={e => setBillingAddr(a => ({ ...a, zip: e.target.value }))} placeholder={t("Zip")} className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                    </div>
                    <input value={billingAddr.country || ''} onChange={e => setBillingAddr(a => ({ ...a, country: e.target.value }))} placeholder={t("Country")} className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                  </div>
                </div>

                {/* Shipping Address */}
                <div>
                  <label className="block text-xs font-medium text-surface-400 mb-1">{t("Shipping Address")}</label>
                  <div className="space-y-1.5">
                    <input value={shippingAddr.line1 || ''} onChange={e => setShippingAddr(a => ({ ...a, line1: e.target.value }))} placeholder={t("Address line 1")} className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                    <input value={shippingAddr.line2 || ''} onChange={e => setShippingAddr(a => ({ ...a, line2: e.target.value }))} placeholder={t("Address line 2")} className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                    <div className="grid grid-cols-3 gap-1.5">
                      <input value={shippingAddr.city || ''} onChange={e => setShippingAddr(a => ({ ...a, city: e.target.value }))} placeholder={t("City")} className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                      <input value={shippingAddr.state || ''} onChange={e => setShippingAddr(a => ({ ...a, state: e.target.value }))} placeholder={t("State")} className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                      <input value={shippingAddr.zip || ''} onChange={e => setShippingAddr(a => ({ ...a, zip: e.target.value }))} placeholder={t("Zip")} className="px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                    </div>
                    <input value={shippingAddr.country || ''} onChange={e => setShippingAddr(a => ({ ...a, country: e.target.value }))} placeholder={t("Country")} className="w-full px-3 py-1.5 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
                  </div>
                </div>

                <button type="submit" className="w-full px-4 py-2 bg-accent-500 text-white rounded-lg hover:bg-accent-600 text-sm font-medium">{t("Save")}</button>
              </form>
            </div>
          ) : (
            <div className="bg-surface-900 border border-surface-800 rounded-lg p-4 space-y-3">
              {customer.email && (
                <div className="flex items-center gap-2 text-sm text-surface-300">
                  <Mail className="h-4 w-4 text-surface-500" />
                  {customer.email}
                </div>
              )}
              {customer.company && (
                <div className="flex items-center gap-2 text-sm text-surface-300">
                  <Building2 className="h-4 w-4 text-surface-500" />
                  {customer.company}
                </div>
              )}
              {customer.phone && (
                <div className="flex items-center gap-2 text-sm text-surface-300">
                  <Phone className="h-4 w-4 text-surface-500" />
                  {customer.phone}
                </div>
              )}
              {customer.notes && (
                <div className="pt-2 border-t border-surface-800">
                  <p className="text-sm text-surface-400">{customer.notes}</p>
                </div>
              )}
              {(customer.billing_address || customer.shipping_address) && (
                <div className="pt-2 border-t border-surface-800 space-y-2">
                  {customer.billing_address && (
                    <div>
                      <div className="flex items-center gap-1 text-xs font-medium text-surface-400 mb-0.5">
                        <MapPin className="h-3 w-3" />

                        {t("Billing")}
                      </div>
                      <div className="text-xs text-surface-300 space-y-0.5">
                        {customer.billing_address.line1 && <div>{customer.billing_address.line1}</div>}
                        {customer.billing_address.line2 && <div>{customer.billing_address.line2}</div>}
                        {(customer.billing_address.city || customer.billing_address.state || customer.billing_address.zip) && (
                          <div>{[customer.billing_address.city, customer.billing_address.state].filter(Boolean).join(', ')} {customer.billing_address.zip}</div>
                        )}
                        {customer.billing_address.country && <div>{customer.billing_address.country}</div>}
                      </div>
                    </div>
                  )}
                  {customer.shipping_address && (
                    <div>
                      <div className="flex items-center gap-1 text-xs font-medium text-surface-400 mb-0.5">
                        <MapPin className="h-3 w-3" />

                        {t("Shipping")}
                      </div>
                      <div className="text-xs text-surface-300 space-y-0.5">
                        {customer.shipping_address.line1 && <div>{customer.shipping_address.line1}</div>}
                        {customer.shipping_address.line2 && <div>{customer.shipping_address.line2}</div>}
                        {(customer.shipping_address.city || customer.shipping_address.state || customer.shipping_address.zip) && (
                          <div>{[customer.shipping_address.city, customer.shipping_address.state].filter(Boolean).join(', ')} {customer.shipping_address.zip}</div>
                        )}
                        {customer.shipping_address.country && <div>{customer.shipping_address.country}</div>}
                      </div>
                    </div>
                  )}
                </div>
              )}
              <div className="pt-2 border-t border-surface-800 text-xs text-surface-500">

                {t("Created")} {new Date(customer.created_at).toLocaleDateString(locale)}
              </div>
            </div>
          )}
        </div>

        {/* Quotes & Orders Tabs */}
        <div className="lg:col-span-2">
          <div className="flex gap-4 mb-4 border-b border-surface-800">
            <button
              onClick={() => setActiveTab('quotes')}
              className={`pb-2 text-sm font-medium transition-colors border-b-2 ${activeTab === 'quotes' ? 'border-accent-500 text-accent-400' : 'border-transparent text-surface-400 hover:text-surface-200'}`}
            >
              <FileText className="h-4 w-4 inline mr-1.5" />

              {t("Quotes (")}{quotes.length})
            </button>
            <button
              onClick={() => setActiveTab('orders')}
              className={`pb-2 text-sm font-medium transition-colors border-b-2 ${activeTab === 'orders' ? 'border-accent-500 text-accent-400' : 'border-transparent text-surface-400 hover:text-surface-200'}`}
            >
              <Package className="h-4 w-4 inline mr-1.5" />

              {t("Orders (")}{orders.length})
            </button>
          </div>

          {activeTab === 'quotes' && (
            <div className="space-y-2">
              <div className="flex justify-end mb-1">
                <button
                  onClick={() => setShowNewQuote(true)}
                  className="inline-flex items-center gap-1.5 text-sm text-accent-400 hover:text-accent-300"
                >
                  <Plus className="h-4 w-4" />

                  {t("New Quote")}
                </button>
              </div>
              {quotes.length === 0 ? (
                <p className="text-sm text-surface-500 py-8 text-center">{t("No quotes for this customer")}</p>
              ) : (
                quotes.map((quote) => (
                  <Link aria-label={t("Open details")}
                    key={quote.id}
                    to={`/quotes/${quote.id}`}
                    className="block bg-surface-900 border border-surface-800 rounded-lg p-4 hover:bg-surface-800/50 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div>
                        <span className="text-sm font-mono text-surface-500 mr-2">{quote.quote_number}</span>
                        <span className="text-sm font-medium text-surface-100">{quote.title}</span>
                      </div>
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${statusColors[quote.status] || ''}`}>
                        {t(displayLabels[quote.status] || quote.status)}
                      </span>
                    </div>
                    <div className="text-xs text-surface-500 mt-1">
                      {new Date(quote.created_at).toLocaleDateString(locale)}
                      {quote.options && quote.options.length > 0 && (
                        <> &middot; {t("{{count}} option", { count: quote.options.length })}</>
                      )}
                    </div>
                  </Link>
                ))
              )}
            </div>
          )}

          {activeTab === 'orders' && (
            <div className="space-y-2">
              {orders.length === 0 ? (
                <p className="text-sm text-surface-500 py-8 text-center">{t("No orders for this customer")}</p>
              ) : (
                orders.map((order) => (
                  <Link aria-label={t("Open details")}
                    key={order.id}
                    to={`/orders/${order.id}`}
                    className="block bg-surface-900 border border-surface-800 rounded-lg p-4 hover:bg-surface-800/50 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <span className="text-sm font-medium text-surface-100">{order.customer_name}</span>
                      <span className="text-xs text-surface-500">{t(displayLabels[order.status] || order.status)}</span>
                    </div>
                    <div className="text-xs text-surface-500 mt-1">
                      {t(displayLabels[order.source] || order.source)} &middot; {new Date(order.created_at).toLocaleDateString(locale)}
                    </div>
                  </Link>
                ))
              )}
            </div>
          )}
        </div>
      </div>

      {/* New Quote Modal */}
      {showNewQuote && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-surface-900 border border-surface-700 rounded-xl p-6 w-full max-w-md">
            <h2 className="text-lg font-display font-semibold text-surface-100 mb-4">{t("New quote for {{name}}", { name: customer.name })}</h2>
            <form onSubmit={handleCreateQuote} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-surface-300 mb-1">{t("Title *")}</label>
                <input name="title" required className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" placeholder={t("e.g. Custom enclosure build")} />
              </div>
              <div>
                <label className="block text-sm font-medium text-surface-300 mb-1">{t("Notes")}</label>
                <textarea name="notes" rows={3} className="w-full px-3 py-2 bg-surface-800 border border-surface-700 rounded-lg text-surface-100 text-sm focus:outline-none focus:ring-1 focus:ring-accent-500" />
              </div>
              <div className="flex justify-end gap-3 pt-2">
                <button type="button" onClick={() => setShowNewQuote(false)} className="px-4 py-2 text-sm text-surface-400 hover:text-surface-200">{t("Cancel")}</button>
                <button type="submit" className="px-4 py-2 bg-accent-500 text-white rounded-lg hover:bg-accent-600 text-sm font-medium">{t("Create Quote")}</button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
