import { InputHTMLAttributes } from 'react';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  /** Shortcut for `className` overrides — merged with the standard Aegis input style. */
  className?: string;
}

const baseClass =
  'w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent';

export default function Input({ className, ...props }: InputProps) {
  return <input className={className ? `${baseClass} ${className}` : baseClass} {...props} />;
}
