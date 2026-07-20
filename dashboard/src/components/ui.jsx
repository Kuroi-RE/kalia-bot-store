export function Button({ variant = 'ghost', className = '', ...props }) {
  const v =
    variant === 'primary'
      ? 'nb-btn-primary'
      : variant === 'danger'
      ? 'nb-btn-danger'
      : variant === 'accent'
      ? 'nb-btn-accent'
      : 'nb-btn-ghost';
  return <button className={`nb-btn ${v} ${className}`} {...props} />;
}

export function Card({ className = '', children }) {
  return <div className={`nb-card p-4 ${className}`}>{children}</div>;
}

export function Input({ label, className = '', ...props }) {
  return (
    <label className="block">
      {label && <span className="nb-label">{label}</span>}
      <input className={`nb-input ${className}`} {...props} />
    </label>
  );
}

export function Textarea({ label, className = '', ...props }) {
  return (
    <label className="block">
      {label && <span className="nb-label">{label}</span>}
      <textarea className={`nb-input ${className}`} rows={4} {...props} />
    </label>
  );
}

export function Select({ label, children, className = '', ...props }) {
  return (
    <label className="block">
      {label && <span className="nb-label">{label}</span>}
      <select className={`nb-input ${className}`} {...props}>
        {children}
      </select>
    </label>
  );
}

const badgeColors = {
  AVAILABLE: 'bg-mint',
  RESERVED: 'bg-brand',
  SOLD: 'bg-ink text-white',
  PENDING: 'bg-brand',
  PAID: 'bg-sky text-white',
  DELIVERED: 'bg-mint',
  EXPIRED: 'bg-white',
  CANCELLED: 'bg-white',
  FAILED: 'bg-blaze text-white',
  SETTLEMENT: 'bg-mint',
  DENIED: 'bg-blaze text-white',
  true: 'bg-mint',
  false: 'bg-white',
};

export function Badge({ value, children, className = '' }) {
  const key = String(value ?? children);
  return <span className={`nb-badge ${badgeColors[key] || 'bg-grape text-white'} ${className}`}>{children ?? String(value)}</span>;
}

export function Spinner({ label = 'Loading…' }) {
  return <div className="py-10 text-center font-bold uppercase tracking-wide animate-pulse">{label}</div>;
}

export function EmptyState({ children }) {
  return (
    <div className="nb-card p-8 text-center font-bold uppercase tracking-wide bg-white/60">{children}</div>
  );
}

export function Modal({ title, open, onClose, children, maxWidth = 'max-w-lg' }) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-ink/40 p-4 py-10">
      <div className={`nb-card w-full ${maxWidth} bg-white shadow-brutal-xl`}>
        <div className="flex items-center justify-between border-b-2 border-ink px-4 py-3">
          <h3 className="text-lg font-bold">{title}</h3>
          <button className="nb-btn nb-btn-danger px-2 py-0.5" onClick={onClose}>
            ✕
          </button>
        </div>
        <div className="p-4">{children}</div>
      </div>
    </div>
  );
}

export function Table({ head, children }) {
  return (
    <div className="nb-card overflow-x-auto bg-white p-0">
      <table className="w-full border-collapse">
        <thead>
          <tr>{head.map((h) => <th key={h} className="nb-th">{h}</th>)}</tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

export function rupiah(amount) {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 0,
  }).format(Number(amount) || 0);
}

export function fmtDate(s) {
  if (!s) return '—';
  return new Date(s).toLocaleString();
}
