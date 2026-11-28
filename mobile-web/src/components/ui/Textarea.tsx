import { forwardRef, useId, type TextareaHTMLAttributes } from 'react';

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  helperText?: string;
  error?: string;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { label, helperText, error, id, className = '', ...rest },
  ref,
) {
  const generatedId = useId();
  const textareaId = id ?? generatedId;
  const helperId = helperText ? `${textareaId}-helper` : undefined;
  const errorId = error ? `${textareaId}-error` : undefined;
  const describedBy = [errorId, helperId].filter(Boolean).join(' ') || undefined;

  return (
    <div className="flex flex-col gap-1.5">
      {label && (
        <label htmlFor={textareaId} className="text-sm font-medium text-[var(--ink)]">
          {label}
          {rest.required && (
            <span className="ml-1 text-[var(--danger-text)]" aria-hidden="true">
              *
            </span>
          )}
        </label>
      )}
      <textarea
        ref={ref}
        id={textareaId}
        aria-invalid={error ? 'true' : undefined}
        aria-describedby={describedBy}
        className={`w-full min-h-[120px] px-3 py-2.5 text-base bg-[var(--surface-light)] text-[var(--ink)] placeholder:text-[var(--muted)] border rounded-[var(--radius-md)] transition-colors duration-[var(--duration-fast)] resize-y ${
          error
            ? 'border-[var(--danger)]'
            : 'border-[var(--surface-border-strong)] hover:border-[var(--ink)]'
        } ${className}`}
        {...rest}
      />
      {error && (
        <p id={errorId} role="alert" className="text-xs font-medium text-[var(--danger-text)]">
          {error}
        </p>
      )}
      {helperText && !error && (
        <p id={helperId} className="text-xs text-[var(--ink-secondary)]">
          {helperText}
        </p>
      )}
    </div>
  );
});
