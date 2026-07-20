import { useEffect, useState } from 'react';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Badge, Modal, Table, Spinner, EmptyState, rupiah, fmtDate } from '../components/ui.jsx';

const STATUSES = ['', 'PENDING', 'PAID', 'DELIVERED', 'EXPIRED', 'CANCELLED', 'FAILED'];

export default function Orders() {
  const toast = useToast();
  const [items, setItems] = useState(null);
  const [status, setStatus] = useState('');
  const [detail, setDetail] = useState(null);
  const [payment, setPayment] = useState(null);

  const load = () => {
    setItems(null);
    api.listOrders({ status: status || undefined, limit: 100 }).then((d) => setItems(d.items || [])).catch((e) => toast.error(e.message));
  };
  useEffect(load, [status]);

  const openDetail = async (o) => {
    setDetail(o);
    setPayment(null);
    try { setPayment(await api.orderPayment(o.id)); } catch { /* maybe none */ }
  };

  const cancel = async (o) => {
    if (!confirm(`Cancel order ${o.order_ref}? Releases its reserved account.`)) return;
    try { await api.cancelOrder(o.id); toast.success('Order cancelled'); setDetail(null); load(); }
    catch (e) { toast.error(e.message); }
  };

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Orders</h1>
      <div className="flex flex-wrap gap-2">
        {STATUSES.map((s) => (
          <button key={s || 'all'} className={`nb-btn px-3 py-1 text-xs ${status === s ? 'nb-btn-accent' : 'nb-btn-ghost'}`} onClick={() => setStatus(s)}>
            {s || 'ALL'}
          </button>
        ))}
      </div>

      {items === null ? <Spinner /> : items.length === 0 ? <EmptyState>No orders</EmptyState> : (
        <Table head={['Ref', 'Amount', 'Status', 'Created', 'Actions']}>
          {items.map((o) => (
            <tr key={o.id}>
              <td className="nb-td font-mono font-bold">{o.order_ref}</td>
              <td className="nb-td">{rupiah(o.amount)}</td>
              <td className="nb-td"><Badge value={o.status} /></td>
              <td className="nb-td text-xs">{fmtDate(o.created_at)}</td>
              <td className="nb-td"><Button className="px-2 py-1 text-xs" onClick={() => openDetail(o)}>View</Button></td>
            </tr>
          ))}
        </Table>
      )}

      <Modal title={detail?.order_ref || 'Order'} open={!!detail} onClose={() => setDetail(null)}>
        {detail && (
          <div className="space-y-2 text-sm">
            <Row k="Status"><Badge value={detail.status} /></Row>
            <Row k="Amount">{rupiah(detail.amount)}</Row>
            <Row k="Order ID">{detail.id}</Row>
            <Row k="Product ID">{detail.product_id}</Row>
            <Row k="Account ID">{detail.account_id ?? '—'}</Row>
            <Row k="Created">{fmtDate(detail.created_at)}</Row>
            <Row k="Paid">{fmtDate(detail.paid_at)}</Row>
            <Row k="Delivered">{fmtDate(detail.delivered_at)}</Row>
            <Row k="Expires">{fmtDate(detail.expires_at)}</Row>
            <div className="mt-3 border-t-2 border-ink pt-2">
              <div className="nb-label">Payment</div>
              {payment ? (
                <div className="space-y-1">
                  <Row k="Gateway">{payment.gateway}</Row>
                  <Row k="Txn ID">{payment.gateway_txn_id || '—'}</Row>
                  <Row k="Status"><Badge value={payment.status} /></Row>
                  <Row k="Amount">{rupiah(payment.gross_amount)}</Row>
                </div>
              ) : <p className="opacity-60">No payment record</p>}
            </div>
            {detail.status === 'PENDING' && (
              <div className="flex justify-end pt-2">
                <Button variant="danger" onClick={() => cancel(detail)}>Cancel Order</Button>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}

function Row({ k, children }) {
  return (
    <div className="flex justify-between gap-4 border-b-2 border-ink/10 py-1">
      <span className="font-bold uppercase tracking-wide opacity-70">{k}</span>
      <span className="text-right">{children}</span>
    </div>
  );
}
