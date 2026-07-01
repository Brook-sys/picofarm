import { CheckCircle, Info, XCircle } from 'lucide-react'
import { cn } from '../lib/utils'

type AppToastTone = 'success' | 'error' | 'info'

export interface AppToastState {
  title: string
  message?: string
  tone?: AppToastTone
}

const toneStyles: Record<AppToastTone, { box: string; icon: string; Icon: typeof CheckCircle }> = {
  success: { box: 'border-emerald-400/50 bg-emerald-500/95 text-white shadow-emerald-500/30', icon: 'bg-white/20 text-white', Icon: CheckCircle },
  error: { box: 'border-red-400/50 bg-red-500/95 text-white shadow-red-500/30', icon: 'bg-white/20 text-white', Icon: XCircle },
  info: { box: 'border-blue-400/50 bg-blue-500/95 text-white shadow-blue-500/30', icon: 'bg-white/20 text-white', Icon: Info },
}

export default function AppToast({ toast, onClose }: { toast: AppToastState; onClose?: () => void }) {
  const tone = toast.tone || 'success'
  const styles = toneStyles[tone]
  const Icon = styles.Icon
  return (
    <div className={cn('fixed bottom-6 right-6 z-50 flex max-w-md animate-pulse items-start gap-3 rounded-2xl border px-5 py-4 text-sm font-semibold shadow-2xl', styles.box)}>
      <div className={cn('mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full', styles.icon)}>
        <Icon className="h-5 w-5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-base font-bold leading-tight">{toast.title}</div>
        {toast.message && <div className="mt-1 text-sm font-medium text-white/85">{toast.message}</div>}
      </div>
      {onClose && <button onClick={onClose} className="ml-2 rounded-full px-2 text-lg leading-none text-white/70 hover:bg-white/10 hover:text-white">×</button>}
    </div>
  )
}
