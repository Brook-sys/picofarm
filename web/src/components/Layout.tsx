import { useTranslation } from 'react-i18next'
import { useState } from 'react'
import { Outlet, NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  FolderKanban,
  ListTodo,
  Printer,
  Package,
  Settings,
  Receipt,
  DollarSign,
  ShoppingBag,
  GanttChart,
  PanelLeftClose,
  PanelLeftOpen,
  FileText,
  Users,
  Camera,
  Archive,
  ListChecks,
  FileCode,
  HardDrive,
  Bell,
} from 'lucide-react'
import { cn } from '../lib/utils'
import AlertBanner from './AlertBanner'
import DispatchNotificationProvider from './DispatchNotificationProvider'
import FeedbackButton from './FeedbackButton'

const navigation = [
  {
    label: 'Production',
    items: [
      { name: 'Queue', href: '/queue', icon: ListChecks },
      { name: 'Files', href: '/files', icon: FileCode },
      { name: 'Printer Files', href: '/printer-files', icon: HardDrive },
      { name: 'Slicer', href: '/slicer', icon: Settings },
      { name: 'Notifications', href: '/notifications', icon: Bell },
      { name: 'Cameras', href: '/cameras', icon: Camera },
      { name: 'Printers', href: '/printers', icon: Printer },
      { name: 'Materials', href: '/materials', icon: Package },
    ],
  },
  {
    label: 'Commerce',
    items: [
      { name: 'Pricing calculator', href: '/pricing', icon: DollarSign },
      { name: 'Quotes', href: '/quotes', icon: FileText },
      { name: 'Customers', href: '/customers', icon: Users },
      { name: 'Projects', href: '/projects', icon: FolderKanban },
      { name: 'Tasks', href: '/tasks', icon: ListTodo },
      { name: 'Archives', href: '/archives', icon: Archive },
      { name: 'Expenses', href: '/expenses', icon: Receipt },
      { name: 'Sales', href: '/sales', icon: DollarSign },
      { name: 'Channels', href: '/channels', icon: ShoppingBag },
      { name: 'Timeline', href: '/timeline', icon: GanttChart },
    ],
  },
]

export default function Layout() {
  const { t } = useTranslation('common')
  const [collapsed, setCollapsed] = useState(() => typeof window !== 'undefined' && window.matchMedia('(max-width: 640px)').matches)

  return (
    <div className="flex h-screen bg-surface-950">
      <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded-lg focus:bg-accent-600 focus:p-3 focus:text-white">{t('Skip to main content')}</a>
      {/* Sidebar */}
      <aside
        className={cn(
          'border-r border-surface-800 bg-surface-900/50 flex flex-col shrink-0 transition-[width] duration-200',
          collapsed ? 'w-16' : 'w-64'
        )}
      >
        {/* Logo + collapse toggle */}
        <div className="h-16 flex items-center border-b border-surface-800 px-3">
          {collapsed ? (
            <div className="relative mx-auto flex h-10 w-10 items-center justify-center">
              <img src="/picofarm_logo.png" alt="PicoFarm" className="h-9 w-9 rounded-md object-contain" />
              <button
                aria-expanded={false}
                aria-controls="main-navigation"
                onClick={() => setCollapsed(false)}
                className="absolute inset-0 flex items-center justify-center rounded-lg bg-surface-900/90 text-surface-100 opacity-0 transition-opacity hover:opacity-100 focus:opacity-100"
                title={t("Expand sidebar")}
                aria-label={t("Expand sidebar")}
              >
                <PanelLeftOpen className="h-5 w-5" />
              </button>
            </div>
          ) : (
            <>
              <div className="flex min-w-0 flex-1 items-center pl-2">
                <img src="/picofarm_logo.png" alt="PicoFarm" className="h-10 w-10 shrink-0 rounded-md object-contain" />
                <span className="ml-3 truncate text-xl font-display font-semibold text-surface-100">
                  PicoFarm
                </span>
              </div>
              <button
                aria-expanded={true}
                aria-controls="main-navigation"
                onClick={() => setCollapsed(true)}
                className="w-10 h-10 flex items-center justify-center rounded-lg text-surface-400 hover:text-surface-100 hover:bg-surface-800 transition-colors"
                title={t("Collapse sidebar")}
                aria-label={t("Collapse sidebar")}
              >
                <PanelLeftClose className="h-5 w-5" />
              </button>
            </>
          )}
        </div>

        {/* Navigation */}
        <nav id="main-navigation" aria-label={t('Main navigation')} className={cn('flex-1 py-4 space-y-1 overflow-y-auto', collapsed ? 'px-2' : 'px-3')}>
          <NavLink
            aria-label={t('Dashboard')}
            aria-current="page"
            to="/dashboard"
            title={collapsed ? t("Dashboard") : undefined}
            className={({ isActive }) =>
              cn(
                'flex items-center rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500 mb-4',
                collapsed ? 'justify-center px-0 py-2' : 'px-3 py-2',
                isActive
                  ? 'bg-accent-500/10 text-accent-400'
                  : 'text-surface-400 hover:text-surface-100 hover:bg-surface-800'
              )
            }
          >
            <LayoutDashboard className={cn('h-4 w-4 shrink-0', !collapsed && 'mr-3')} />
            {!collapsed && t("Dashboard")}
          </NavLink>
          {navigation.map((section) => (
            <div key={section.label} className="mb-3">
              {!collapsed && <div className="px-3 mb-1 text-[10px] font-semibold tracking-widest text-surface-600">{t(section.label)}</div>}
              {section.items.map((item) => (
                <NavLink
                  key={item.name}
                  aria-label={t(item.name)}
                  aria-current="page"
                  to={item.href}
                  title={collapsed ? t(item.name) : undefined}
                  className={({ isActive }) =>
                    cn(
                      'flex items-center rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500',
                      collapsed ? 'justify-center px-0 py-2' : 'px-3 py-2',
                      isActive
                        ? 'bg-accent-500/10 text-accent-400'
                        : 'text-surface-400 hover:text-surface-100 hover:bg-surface-800'
                    )
                  }
                >
                  <item.icon className={cn('h-4 w-4 shrink-0', !collapsed && 'mr-3')} />
                  {!collapsed && t(item.name)}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>

        {/* Settings */}
        <div className={cn('py-4 border-t border-surface-800', collapsed ? 'px-2' : 'px-3')}>
          <NavLink
            aria-label={t('Settings')}
            aria-current="page"
            to="/settings"
            title={collapsed ? t("Settings") : undefined}
            className={({ isActive }) =>
              cn(
                'flex items-center rounded-lg text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-500',
                collapsed ? 'justify-center px-0 py-2.5' : 'px-3 py-2.5',
                isActive
                  ? 'bg-accent-500/10 text-accent-400'
                  : 'text-surface-400 hover:text-surface-100 hover:bg-surface-800'
              )
            }
          >
            <Settings className={cn('h-5 w-5 shrink-0', !collapsed && 'mr-3')} />
            {!collapsed && t("Settings")}
          </NavLink>
        </div>
      </aside>

      {/* Main content */}
      <main id="main-content" tabIndex={-1} className="flex-1 overflow-auto min-w-0">
        {/* Operational alerts (low spools, orders due). The component and its
            API existed and worked; nothing ever rendered it, so no alert was
            ever seen by a user. */}
        <div className="px-6 pt-4">
          <AlertBanner />
        </div>
        <Outlet />
      </main>

      {/* Global dispatch notification */}
      <DispatchNotificationProvider />

      {/* Beta feedback button */}
      <FeedbackButton />
    </div>
  )
}
