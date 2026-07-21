import { useEffect, useState } from 'react';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Card, Badge, Table, Spinner, EmptyState, rupiah, fmtDate } from '../components/ui.jsx';

export default function Confirmations() {
  const toast = useToast();
  const [items, setItems] = useState(null);
  const [config, setConfig] = useState(null);
  const [busyId, setBusyId] = useState(null);

  const load = () => {
    setItems(null);
    api.listOrders({ status: 'PENDING', limit: 100 })
      .then((d) => setItems(d.items || []))
      .catch((e) => toast.error(e.message));
  };
  useEffect(() => {
    api.getConfig().then(setConfig).catch(() => {});
    load();
  }, []);

  const confirm = async (o) => {
    if (!confirm_ok(o)) return;
    setBusyId(o.id);
    try {
      const res = await api.confirmPayment(o.id);
      toast.success(res.confirmed ? `Confirmed ${o.order_ref} — delivering` : 'Order was not pending');
      load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusyId(null);
    }
  };

  const cancel = async (o) => {
    if (!window.confirm(`Cancel ${o.order_ref}? Releases its reserved account.`)) return;
    setBusyId(o.id);
    try {
      await api.cancelOrder(o.id);
      toast.success('Order cancelled');
      load();
    } catch (e) {
      toast.error(e.message);
    } finally {
      setBusyId(null);
    }
  };

  const confirm_ok = (o) =>
    window.confirm(`Confirm payment received for ${o.order_ref} (${rupiah(o.amount)})?\nOnly do this after you've verified the funds arrived.`);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Payment Confirmations</h1>
        <Button onClick={load}>↻ Refresh</Button>
      </div>

      <Card className={config?.requires_manual_confirmation ? 'bg-brand' : 'bg-white'}>
        <p className="text-sm font-bold">
          Provider: {config?.payment_provider || '…'}
          {config?.requires_manual_confirmation
            ? ' — payments are confirmed manually here after you verify funds in your account.'
            : ' — payments settle automatically; use this only as an override.'}
        </p>
      </Card>

      {items === null ? (
        <Spinner />
      ) : items.length === 0 ? (
        <EmptyState>No pending orders</EmptyState>
      ) : (
        <Table head={['Ref', 'Amount', 'Status', 'Created', 'Expires', 'Actions']}>
          {items.map((o) => (
            <tr key={o.id}>
              <td className="nb-td font-mono font-bold">{o.order_ref}</td>
              <td className="nb-td font-bold">{rupiah(o.amount)}</td>
              <td className="nb-td"><Badge value={o.status} /></td>
              <td className="nb-td text-xs">{fmtDate(o.created_at)}</td>
              <td className="nb-td text-xs">{fmtDate(o.expires_at)}</td>
              <td className="nb-td">
                <div className="flex flex-wrap gap-1">
                  <Button variant="primary" className="px-2 py-1 text-xs" disabled={busyId === o.id} onClick={() => confirm(o)}>
                    ✅ Confirm Paid
                  </Button>
                  <Button variant="danger" className="px-2 py-1 text-xs" disabled={busyId === o.id} onClick={() => cancel(o)}>
                    Cancel
                  </Button>
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}
    </div>
  );
}
