import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';

export interface ToastItem {
  id: string;
  message: string;
  variant: 'info' | 'warning' | 'danger' | 'success';
  countdownSeconds?: number;
}

interface ToastContextValue {
  pushToast: (toast: Omit<ToastItem, 'id'>) => void;
  dismissToast: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const pushToast = useCallback((toast: Omit<ToastItem, 'id'>) => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    setToasts((prev) => [...prev.slice(-2), { ...toast, id }]);
  }, []);

  return (
    <ToastContext.Provider value={{ pushToast, dismissToast }}>
      {children}
      <div
        aria-live="polite"
        aria-atomic="false"
        className="fixed bottom-20 md:bottom-6 right-4 z-50 flex flex-col gap-2 max-w-sm w-[calc(100vw-2rem)] pointer-events-none"
      >
        {toasts.map((toast) => (
          <ToastBanner key={toast.id} toast={toast} onDismiss={dismissToast} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastBanner({
  toast,
  onDismiss,
}: {
  toast: ToastItem;
  onDismiss: (id: string) => void;
}) {
  const [remaining, setRemaining] = useState<number>(toast.countdownSeconds ?? 0);

  useEffect(() => {
    if (toast.countdownSeconds && toast.countdownSeconds > 0) {
      const interval = window.setInterval(() => {
        setRemaining((prev) => {
          if (prev <= 1) {
            window.clearInterval(interval);
            onDismiss(toast.id);
            return 0;
          }
          return prev - 1;
        });
      }, 1000);
      return () => window.clearInterval(interval);
    }

    const timer = window.setTimeout(() => onDismiss(toast.id), 5000);
    return () => window.clearTimeout(timer);
  }, [toast.id, toast.countdownSeconds, onDismiss]);

  const variantClasses: Record<ToastItem['variant'], string> = {
    info: 'bg-[var(--surface-dark)] text-[var(--ink-on-dark)] border-[var(--border)]',
    success: 'bg-[var(--t1-text)] text-[var(--ink-on-dark)] border-[var(--t1-border)]',
    warning: 'bg-[var(--t3-bg)] text-[var(--t3-text)] border-[var(--t3-border)]',
    danger: 'bg-[var(--danger-bg)] text-[var(--danger-text)] border-[var(--danger-border)]',
  };

  return (
    <div
      role={toast.variant === 'danger' || toast.variant === 'warning' ? 'alert' : 'status'}
      className={`pointer-events-auto flex items-start justify-between gap-3 px-4 py-3 text-sm border rounded-[var(--radius-md)] shadow-lg ${variantClasses[toast.variant]}`}
    >
      <p className="m-0 leading-snug">
        {toast.message}
        {remaining > 0 && (
          <span className="ml-1.5 font-[var(--font-mono)] font-semibold">
            (retry in {remaining}s)
          </span>
        )}
      </p>
      <button
        type="button"
        onClick={() => onDismiss(toast.id)}
        aria-label="Dismiss notification"
        className="shrink-0 min-w-[28px] min-h-[28px] flex items-center justify-center opacity-80 hover:opacity-100 cursor-pointer"
      >
        &times;
      </button>
    </div>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return ctx;
}
