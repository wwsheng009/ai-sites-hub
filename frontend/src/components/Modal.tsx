import { useEffect } from 'react'
import type { ReactNode } from 'react'

/** 通用模态框：遮罩点击关闭 + Esc 关闭 + 滚动锁定 */
export default function Modal({
  open,
  title,
  onClose,
  children,
  footer,
  width = 'max-w-lg',
}: {
  open: boolean
  title: ReactNode
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  width?: string
}) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className={`modal-panel ${width}`} onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true">
        <div className="card-header flex items-center justify-between">
          <h3 className="font-semibold text-gray-900 dark:text-white">{title}</h3>
          <button className="btn btn-ghost btn-sm" onClick={onClose} aria-label="关闭">
            ✕
          </button>
        </div>
        <div className="card-body">{children}</div>
        {footer && <div className="card-footer flex justify-end gap-2">{footer}</div>}
      </div>
    </div>
  )
}
