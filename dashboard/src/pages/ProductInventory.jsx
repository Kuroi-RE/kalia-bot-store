import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Card, Badge, Modal, Table, Spinner, EmptyState, Input, fmtDate } from '../components/ui.jsx';

export default function ProductInventory() {
  const { id } = useParams();
  const navigate = useNavigate();
  const toast = useToast();
  const [product, setProduct] = useState(null);
  const [summary, setSummary] = useState(null);
  const [accounts, setAccounts] = useState(null);
  const [statusFilter, setStatusFilter] = useState('');
  const [addOpen, setAddOpen] = useState(false);
  const [creds, setCreds] = useState({});
  const [saving, setSaving] = useState(false);

  const loadProduct = () => api.getProduct(id).then(setProduct).catch((e) => toast.error(e.message));
  const loadSummary = () => api.inventorySummary(id).then(setSummary).catch(() => {});
  const loadAccounts = () => {
    setAccounts(null);
    api.listAccounts(id, { status: statusFilter || undefined, limit: 200 })
      .then((d) => setAccounts(d.items || []))
      .catch((e) => toast.error(e.message));
  };

  useEffect(() => { loadProduct(); loadSummary(); }, [id]);
  useEffect(loadAccounts, [id, statusFilter]);

  const openAdd = () => {
    const init = {};
    (product?.credential_schema || []).forEach((f) => (init[f.key] = ''));
    setCreds(init);
    setAddOpen(true);
  };

  const addAccount = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.createAccounts(id, { credentials: creds });
      toast.success('Account added');
      setAddOpen(false);
      loadSummary();
      loadAccounts();
    } catch (err) {
      toast.error(err.message);
    } finally {
      setSaving(false);
    }
  };

  const removeAccount = async (a) => {
    if (!confirm(`Delete account #${a.id}?`)) return;
    try {
      await api.deleteAccount(a.id);
      toast.success('Account deleted');
      loadSummary();
      loadAccounts();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const schema = product?.credential_schema || [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <button className="mb-1 text-sm font-bold underline" onClick={() => navigate('/products')}>← Products</button>
          <h1 className="text-2xl font-bold">{product ? product.name : 'Inventory'}</h1>
        </div>
        <Button variant="primary" onClick={openAdd}>+ Add Account</Button>
      </div>

      {summary && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            ['Available', summary.available, 'bg-mint'],
            ['Reserved', summary.reserved, 'bg-brand'],
            ['Sold', summary.sold, 'bg-ink text-white'],
            ['Total', summary.total, 'bg-white'],
          ].map(([label, val, bg]) => (
            <Card key={label} className={`${bg} text-center`}>
              <div className="text-3xl font-bold">{val}</div>
              <div className="text-xs font-bold uppercase tracking-wide">{label}</div>
            </Card>
          ))}
        </div>
      )}

      <div className="flex gap-2">
        {['', 'AVAILABLE', 'RESERVED', 'SOLD'].map((s) => (
          <button
            key={s || 'all'}
            className={`nb-btn px-3 py-1 text-xs ${statusFilter === s ? 'nb-btn-accent' : 'nb-btn-ghost'}`}
            onClick={() => setStatusFilter(s)}
          >
            {s || 'ALL'}
          </button>
        ))}
      </div>

      {accounts === null ? (
        <Spinner />
      ) : accounts.length === 0 ? (
        <EmptyState>No accounts</EmptyState>
      ) : (
        <Table head={['ID', 'Credentials', 'Status', 'Created', 'Actions']}>
          {accounts.map((a) => (
            <tr key={a.id}>
              <td className="nb-td font-bold">#{a.id}</td>
              <td className="nb-td font-mono text-xs">
                {Object.entries(a.credentials || {}).map(([k, v]) => (
                  <div key={k}><span className="opacity-60">{k}:</span> {String(v)}</div>
                ))}
              </td>
              <td className="nb-td"><Badge value={a.status} /></td>
              <td className="nb-td text-xs">{fmtDate(a.created_at)}</td>
              <td className="nb-td">
                <Button variant="danger" className="px-2 py-1 text-xs" onClick={() => removeAccount(a)}>Del</Button>
              </td>
            </tr>
          ))}
        </Table>
      )}

      <Modal title="Add Account" open={addOpen} onClose={() => setAddOpen(false)}>
        <form onSubmit={addAccount} className="space-y-3">
          {schema.length === 0 && <p className="text-sm opacity-70">This product has no credential schema. Add fields on the product first, or enter a custom key below.</p>}
          {schema.map((f) => (
            <Input
              key={f.key}
              label={`${f.label || f.key}${f.required ? ' *' : ''}`}
              value={creds[f.key] || ''}
              onChange={(e) => setCreds({ ...creds, [f.key]: e.target.value })}
              required={f.required}
            />
          ))}
          {schema.length === 0 && (
            <>
              <Input label="Field key" onChange={(e) => setCreds({ [e.target.value]: creds[Object.keys(creds)[0]] || '' })} placeholder="e.g. email" />
            </>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => setAddOpen(false)}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={saving}>{saving ? 'Saving…' : 'Add'}</Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
