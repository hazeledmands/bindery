// Toggle is the small switch control used across several Settings tabs.

export default function Toggle({ checked, onChange, title, disabled }: { checked: boolean; onChange: () => void; title?: string; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={onChange}
      title={title}
      disabled={disabled}
      className={`relative w-9 h-5 rounded-full transition-colors flex-shrink-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:ring-offset-1 dark:focus-visible:ring-offset-zinc-900 disabled:opacity-50 disabled:cursor-not-allowed ${
        checked ? 'bg-emerald-600' : 'bg-slate-300 dark:bg-zinc-700'
      }`}
    >
      <span className={`absolute top-0.5 left-0.5 w-4 h-4 bg-white rounded-full shadow-sm transition-transform duration-150 ${checked ? 'translate-x-4' : ''}`} />
    </button>
  )
}
