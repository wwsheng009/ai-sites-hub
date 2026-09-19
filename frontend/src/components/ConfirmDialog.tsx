import Modal from './Modal'

/** 危险操作确认对话框（红色主题，用于删除等不可逆操作） */
export default function ConfirmDialog({
  open,
  title,
  message,
  confirmText = '确认删除',
  busy = false,
  onConfirm,
  onCancel,
}: {
  open: boolean
  title: string
  message: ReactNodeLike
  confirmText?: string
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}) {
  return (
    <Modal
      open={open}
      title={title}
      onClose={onCancel}
      width="max-w-md"
      footer={
        <>
          <button className="btn btn-secondary" onClick={onCancel} disabled={busy}>
            取消
          </button>
          <button className="btn btn-danger" onClick={onConfirm} disabled={busy}>
            {busy ? '处理中…' : confirmText}
          </button>
        </>
      }
    >
      <div className="flex items-start gap-3">
        <span className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-red-100 text-xl dark:bg-red-900/30">
          ⚠️
        </span>
        <div className="min-w-0 text-sm text-gray-700 dark:text-gray-300">{message}</div>
      </div>
    </Modal>
  )
}

type ReactNodeLike = import('react').ReactNode
