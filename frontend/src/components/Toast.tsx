import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'

export type ToastKind = 'success' | 'error' | 'info' | 'warning'

interface ToastItem {
  id: number
  kind: ToastKind
  message: string
}

interface ToastContextValue {
  push: (kind: ToastKind, message: string) => void
  success: (message: string) => void
  error: (message: string) => void
  info: (message: string) => void
  warning: (message: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

const kindStyle: Record<ToastKind, { icon: string; cls: string }> = {
  success: { icon: '✓', cls: 'border-emerald-300/60 dark:border-emerald-700/60' },
  error: { icon: '✕', cls: 'border-red-300/60 dark:border-red-700/60' },
  warning: { icon: '!', cls: 'border-amber-300/60 dark:border-amber-700/60' },
  info: { icon: 'i', cls: 'border-sky-300/60 dark:border-sky-700/60' },
}

const kindIconBg: Record<ToastKind, string> = {
  success: 'bg-emerald-500',
  error: 'bg-red-500',
  warning: 'bg-amber-500',
  info: 'bg-sky-500',
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const idRef = useRef(0)
  const timers = useRef<Map<number, ReturnType<typeof setTimeout>>>(new Map())

  const remove = useCallback((id: number) => {
    setItems((arr) => arr.filter((t) => t.id !== id))
    const timer = timers.current.get(id)
    if (timer) {
      clearTimeout(timer)
      timers.current.delete(id)
    }
  }, [])

  const push = useCallback(
    (kind: ToastKind, message: string) => {
      const id = ++idRef.current
      setItems((arr) => [...arr.slice(-4), { id, kind, message }])
      timers.current.set(
        id,
        setTimeout(() => remove(id), 4200),
      )
    },
    [remove],
  )

  useEffect(() => () => timers.current.forEach((t) => clearTimeout(t)), [])

  const value: ToastContextValue = {
    push,
    success: (m) => push('success', m),
    error: (m) => push('error', m),
    info: (m) => push('info', m),
    warning: (m) => push('warning', m),
  }

  return (
    <ToastContext.Provider value={value}>
      {children}
      {/* Toast 容器：右上角，slide-in-right 动画 */}
      <div className="pointer-events-none fixed right-4 top-20 z-[60] flex w-full max-w-sm flex-col gap-2">
        {items.map((t) => {
          const s = kindStyle[t.kind]
          return (
            <div
              key={t.id}
              className={`card pointer-events-auto flex items-start gap-3 border p-4 shadow-glass ${s.cls} animate-slide-up`}
              role="status"
            >
              <span
                className={`flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold text-white ${kindIconBg[t.kind]}`}
              >
                {s.icon}
              </span>
              <p className="min-w-0 flex-1 break-words text-sm text-gray-800 dark:text-gray-200">{t.message}</p>
              <button
                className="text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-200"
                onClick={() => remove(t.id)}
                aria-label="关闭通知"
              >
                ✕
              </button>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast 必须在 <ToastProvider> 内使用')
  return ctx
}

/** 将 ApiError/Error 转为可读文本 */
export function errMsg(e: unknown): string {
  return String((e as Error)?.message ?? e)
}
