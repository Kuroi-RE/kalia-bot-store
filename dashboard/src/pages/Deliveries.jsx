import { useEffect, useState } from 'react';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Badge, Table, Spinner, EmptyState, fmtDate } from '../components/ui.jsx';

const STATUSES = ['', 'PENDING', 'DELIVERED', 'FAILED'];

export default function Deliveries() {
  const toast = useToast();
  const [items, setItems] = useState(null);
  const [status, setStatus] = useState('');

  const load = () => {
    setItems(null);
    api.listDeliveries({ status: status || undefined, limit: 100 }).then((d) => setItems(d.items || [])).catch((e) => toast.error(e.message));
  };
  useEffect(load, [status]);

  const redeliver = async (d) => {
    try { await api.redeliver(d.order_id); toast.success('Redelivery triggered'); load(); }
    catch (e) { toast.error(e.message); }
  };

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Deliveries</h1>
      <div className="flex flex-wrap gap-2">
        {STATUSES.map((s) => (
          <button key={s || 'all'} className={`nb-btn px-3 py-1 text-xs ${status === s ? 'nb-btn-accent' : 'nb-btn-ghost'}`} onClick={() => setStatus(s)}>
            {s || 'ALL'}
          </button>
        ))}
      </div>

      {items === null ? <Spinner /> : items.length === 0 ? <EmptyState>No deliveries</EmptyState> : (
        <Table head={['ID', 'Order', 'Account', 'Status', 'Attempts', 'Last error', 'Actions']}>
          {items.map((d) => (
            <tr key={d.id}>
              <td className="nb-td font-bold">#{d.id}</td>
              <td className="nb-td">{d.order_id}</td>
              <td className="nb-td">{d.account_id}</td>
              <td className="nb-td"><Badge value={d.status} /></td>
              <td className="nb-td">{d.attempts}</td>
              <td className="nb-td max-w-xs truncate text-xs text-blaze">{d.last_error || '—'}</td>
              <td className="nb-td">
                {d.status !== 'DELIVERED' && (
                  <Button variant="accent" className="px-2 py-1 text-xs" onClick={() => redeliver(d)}>Redeliver</Button>
                )}
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  );
}
