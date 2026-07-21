import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Card, Input, Textarea, Badge, Modal, Table, Spinner, EmptyState, rupiah } from '../components/ui.jsx';

const TYPE_FIELD = { key: 'type', label: 'Type', type: 'string', required: true };
const emptyForm = { name: '', description: '', base_price: 0, is_active: true, credential_schema: [{ ...TYPE_FIELD }] };

// ensureTypeField guarantees a required "type" field exists at the front of the
// schema (older products created before this rule may lack it).
function ensureTypeField(schema = []) {
  const rest = schema.filter((f) => f.key !== 'type');
  return [{ ...TYPE_FIELD }, ...rest];
}

function SchemaEditor({ schema, onChange }) {
  const update = (i, patch) => onChange(schema.map((f, idx) => (idx === i ? { ...f, ...patch } : f)));
  const add = () => onChange([...schema, { key: '', label: '', type: 'string', required: true }]);
  const remove = (i) => onChange(schema.filter((_, idx) => idx !== i));

  return (
    <div>
      <div className="nb-label flex items-center justify-between">
        <span>Credential fields</span>
        <Button type="button" onClick={add} className="px-2 py-0.5 text-xs">+ Field</Button>
      </div>
      <div className="space-y-2">
        {schema.map((f, i) => {
          const locked = f.key === 'type';
          return (
            <div key={i} className="grid grid-cols-12 gap-2 items-center">
              <input
                className="nb-input col-span-3"
                placeholder="key"
                value={f.key}
                disabled={locked}
                title={locked ? 'The "type" field is required and cannot be renamed or removed.' : undefined}
                onChange={(e) => update(i, { key: e.target.value })}
              />
              <input className="nb-input col-span-3" placeholder="label" value={f.label} onChange={(e) => update(i, { label: e.target.value })} />
              <select className="nb-input col-span-3" value={f.type} onChange={(e) => update(i, { type: e.target.value })}>
                <option value="string">string</option>
                <option value="secret">secret</option>
                <option value="url">url</option>
                <option value="text">text</option>
              </select>
              <label className="col-span-2 flex items-center gap-1 text-xs font-bold">
                <input type="checkbox" checked={!!f.required} disabled={locked} onChange={(e) => update(i, { required: e.target.checked })} /> req
              </label>
              {locked ? (
                <span className="col-span-1 text-center text-lg" title="Required field">🔒</span>
              ) : (
                <button type="button" className="nb-btn nb-btn-danger col-span-1 px-2 py-1" onClick={() => remove(i)}>✕</button>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default function Products() {
  const toast = useToast();
  const navigate = useNavigate();
  const [items, setItems] = useState(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState(null);

  const load = () => {
    setItems(null);
    api.listProducts({ limit: 200 }).then((d) => setItems(d.items || [])).catch((e) => toast.error(e.message));
  };
  useEffect(load, []);

  const openNew = () => {
    setEditing(null);
    setForm(emptyForm);
    setModalOpen(true);
  };
  const openEdit = (p) => {
    setEditing(p);
    setForm({
      name: p.name,
      description: p.description || '',
      base_price: p.base_price,
      is_active: p.is_active,
      credential_schema: ensureTypeField(p.credential_schema || []),
    });
    setModalOpen(true);
  };

  const save = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      const body = {
        name: form.name,
        description: form.description,
        base_price: Number(form.base_price),
        is_active: form.is_active,
        credential_schema: ensureTypeField(form.credential_schema),
      };
      if (editing) {
        await api.updateProduct(editing.id, body);
        toast.success('Product updated');
      } else {
        await api.createProduct(body);
        toast.success('Product created');
      }
      setModalOpen(false);
      load();
    } catch (err) {
      toast.error(err.message);
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (p) => {
    try {
      await api.setProductStatus(p.id, !p.is_active);
      load();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const remove = async (p) => {
    try {
      await api.deleteProduct(p.id);
      toast.success('Product deleted');
      load();
    } catch (e) {
      if (e.status === 409) {
        setConflict(p); // offer disable / force delete
        return;
      }
      toast.error(e.message);
    }
  };

  const forceRemove = async () => {
    const p = conflict;
    setConflict(null);
    try {
      await api.deleteProduct(p.id, true);
      toast.success('Product and its accounts deleted');
      load();
    } catch (e) {
      toast.error(e.message);
    }
  };

  const disableFromConflict = async () => {
    const p = conflict;
    setConflict(null);
    try {
      await api.setProductStatus(p.id, false);
      toast.success('Product disabled');
      load();
    } catch (e) {
      toast.error(e.message);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Products</h1>
        <Button variant="primary" onClick={openNew}>+ New Product</Button>
      </div>

      {items === null ? (
        <Spinner />
      ) : items.length === 0 ? (
        <EmptyState>No products yet</EmptyState>
      ) : (
        <Table head={['Name', 'Price', 'Fields', 'Status', 'Actions']}>
          {items.map((p) => (
            <tr key={p.id}>
              <td className="nb-td font-bold">{p.name}</td>
              <td className="nb-td">{rupiah(p.base_price)}</td>
              <td className="nb-td">{(p.credential_schema || []).length}</td>
              <td className="nb-td"><Badge value={p.is_active}>{p.is_active ? 'ACTIVE' : 'DISABLED'}</Badge></td>
              <td className="nb-td">
                <div className="flex flex-wrap gap-1">
                  <Button className="px-2 py-1 text-xs" onClick={() => navigate(`/products/${p.id}/inventory`)}>Inventory</Button>
                  <Button className="px-2 py-1 text-xs" onClick={() => openEdit(p)}>Edit</Button>
                  <Button className="px-2 py-1 text-xs" onClick={() => toggle(p)}>{p.is_active ? 'Disable' : 'Enable'}</Button>
                  <Button variant="danger" className="px-2 py-1 text-xs" onClick={() => remove(p)}>Del</Button>
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}

      <Modal title={editing ? 'Edit Product' : 'New Product'} open={modalOpen} onClose={() => setModalOpen(false)} maxWidth="max-w-2xl">
        <form onSubmit={save} className="space-y-4">
          <Input label="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
          <Textarea label="Description" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} rows={2} />
          <Input label="Base price (IDR)" type="number" min="0" value={form.base_price} onChange={(e) => setForm({ ...form, base_price: e.target.value })} required />
          <label className="flex items-center gap-2 font-bold">
            <input type="checkbox" checked={form.is_active} onChange={(e) => setForm({ ...form, is_active: e.target.checked })} /> Active
          </label>
          <Card className="bg-paper">
            <SchemaEditor schema={form.credential_schema} onChange={(s) => setForm({ ...form, credential_schema: s })} />
          </Card>
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => setModalOpen(false)}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={saving}>{saving ? 'Saving…' : 'Save'}</Button>
          </div>
        </form>
      </Modal>

      <Modal title="Can't delete product" open={!!conflict} onClose={() => setConflict(null)}>
        <div className="space-y-4">
          <p className="font-medium">
            <b>{conflict?.name}</b> still has inventory accounts, so it can't be deleted directly.
          </p>
          <div className="nb-card bg-paper p-3 text-sm">
            <p className="font-bold">Choose one:</p>
            <ul className="mt-1 list-disc pl-5 space-y-1">
              <li><b>Disable</b> — hides it from the bot but keeps all history (recommended).</li>
              <li><b>Force delete</b> — permanently removes the product, its accounts, and <b>all related orders, payments and deliveries</b>. Use only for test/mistake data.</li>
            </ul>
          </div>
          <div className="flex flex-wrap justify-end gap-2">
            <Button onClick={() => setConflict(null)}>Cancel</Button>
            <Button variant="accent" onClick={disableFromConflict}>Disable instead</Button>
            <Button variant="danger" onClick={forceRemove}>Force delete</Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
