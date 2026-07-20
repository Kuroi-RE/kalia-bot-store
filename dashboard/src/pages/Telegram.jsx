import { useEffect, useState } from 'react';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Badge, Modal, Table, Spinner, EmptyState, Input, Textarea } from '../components/ui.jsx';

export default function Telegram() {
  const [tab, setTab] = useState('responses');
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Telegram Content</h1>
      <div className="flex gap-2">
        <button className={`nb-btn ${tab === 'responses' ? 'nb-btn-accent' : 'nb-btn-ghost'}`} onClick={() => setTab('responses')}>
          Responses
        </button>
        <button className={`nb-btn ${tab === 'menus' ? 'nb-btn-accent' : 'nb-btn-ghost'}`} onClick={() => setTab('menus')}>
          Menus
        </button>
      </div>
      {tab === 'responses' ? <Responses /> : <Menus />}
    </div>
  );
}

function Responses() {
  const toast = useToast();
  const [items, setItems] = useState(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState({ command: '', reply_text: '', is_enabled: true });
  const [saving, setSaving] = useState(false);

  const load = () => {
    setItems(null);
    api.listResponses().then((d) => setItems(d.items || [])).catch((e) => toast.error(e.message));
  };
  useEffect(load, []);

  const openNew = () => { setEditing(null); setForm({ command: '', reply_text: '', is_enabled: true }); setOpen(true); };
  const openEdit = (r) => { setEditing(r); setForm({ command: r.command, reply_text: r.reply_text, is_enabled: r.is_enabled }); setOpen(true); };

  const save = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      if (editing) { await api.updateResponse(editing.id, { command: form.command, reply_text: form.reply_text }); toast.success('Response updated'); }
      else { await api.createResponse(form); toast.success('Response created'); }
      setOpen(false); load();
    } catch (err) { toast.error(err.message); } finally { setSaving(false); }
  };
  const toggle = async (r) => { try { await api.setResponseStatus(r.id, !r.is_enabled); load(); } catch (e) { toast.error(e.message); } };
  const remove = async (r) => { if (!confirm(`Delete ${r.command}?`)) return; try { await api.deleteResponse(r.id); toast.success('Deleted'); load(); } catch (e) { toast.error(e.message); } };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-bold opacity-70">Static replies (e.g. /testimoni, /contact, /bantuan)</p>
        <Button variant="primary" onClick={openNew}>+ New Response</Button>
      </div>
      {items === null ? <Spinner /> : items.length === 0 ? <EmptyState>No responses</EmptyState> : (
        <Table head={['Command', 'Reply', 'Status', 'Actions']}>
          {items.map((r) => (
            <tr key={r.id}>
              <td className="nb-td font-mono font-bold">{r.command}</td>
              <td className="nb-td max-w-md truncate text-sm">{r.reply_text}</td>
              <td className="nb-td"><Badge value={r.is_enabled}>{r.is_enabled ? 'ON' : 'OFF'}</Badge></td>
              <td className="nb-td">
                <div className="flex gap-1">
                  <Button className="px-2 py-1 text-xs" onClick={() => openEdit(r)}>Edit</Button>
                  <Button className="px-2 py-1 text-xs" onClick={() => toggle(r)}>{r.is_enabled ? 'Off' : 'On'}</Button>
                  <Button variant="danger" className="px-2 py-1 text-xs" onClick={() => remove(r)}>Del</Button>
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}
      <Modal title={editing ? 'Edit Response' : 'New Response'} open={open} onClose={() => setOpen(false)}>
        <form onSubmit={save} className="space-y-3">
          <Input label="Command" value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })} placeholder="/testimoni" required />
          <Textarea label="Reply text" value={form.reply_text} onChange={(e) => setForm({ ...form, reply_text: e.target.value })} rows={5} />
          {!editing && (
            <label className="flex items-center gap-2 font-bold">
              <input type="checkbox" checked={form.is_enabled} onChange={(e) => setForm({ ...form, is_enabled: e.target.checked })} /> Enabled
            </label>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={saving}>{saving ? 'Saving…' : 'Save'}</Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}

function Menus() {
  const toast = useToast();
  const [items, setItems] = useState(null);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState({ command: '', title: '', reply_text: '', sort_order: 0, is_enabled: true });
  const [saving, setSaving] = useState(false);

  const load = () => {
    setItems(null);
    api.listMenus().then((d) => setItems(d.items || [])).catch((e) => toast.error(e.message));
  };
  useEffect(load, []);

  const openNew = () => { setEditing(null); setForm({ command: '', title: '', reply_text: '', sort_order: 0, is_enabled: true }); setOpen(true); };
  const openEdit = (m) => { setEditing(m); setForm({ command: m.command, title: m.title, reply_text: m.reply_text, sort_order: m.sort_order, is_enabled: m.is_enabled }); setOpen(true); };

  const save = async (e) => {
    e.preventDefault();
    setSaving(true);
    try {
      const body = { command: form.command, title: form.title, reply_text: form.reply_text, sort_order: Number(form.sort_order) };
      if (editing) { await api.updateMenu(editing.id, body); toast.success('Menu updated'); }
      else { await api.createMenu({ ...body, is_enabled: form.is_enabled }); toast.success('Menu created'); }
      setOpen(false); load();
    } catch (err) { toast.error(err.message); } finally { setSaving(false); }
  };
  const toggle = async (m) => { try { await api.setMenuStatus(m.id, !m.is_enabled); load(); } catch (e) { toast.error(e.message); } };
  const remove = async (m) => { if (!confirm(`Delete ${m.command}?`)) return; try { await api.deleteMenu(m.id); toast.success('Deleted'); load(); } catch (e) { toast.error(e.message); } };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-bold opacity-70">Command menus (can trigger the order flow)</p>
        <Button variant="primary" onClick={openNew}>+ New Menu</Button>
      </div>
      {items === null ? <Spinner /> : items.length === 0 ? <EmptyState>No menus</EmptyState> : (
        <Table head={['Order', 'Command', 'Title', 'Status', 'Actions']}>
          {items.map((m) => (
            <tr key={m.id}>
              <td className="nb-td font-bold">{m.sort_order}</td>
              <td className="nb-td font-mono font-bold">{m.command}</td>
              <td className="nb-td">{m.title}</td>
              <td className="nb-td"><Badge value={m.is_enabled}>{m.is_enabled ? 'ON' : 'OFF'}</Badge></td>
              <td className="nb-td">
                <div className="flex gap-1">
                  <Button className="px-2 py-1 text-xs" onClick={() => openEdit(m)}>Edit</Button>
                  <Button className="px-2 py-1 text-xs" onClick={() => toggle(m)}>{m.is_enabled ? 'Off' : 'On'}</Button>
                  <Button variant="danger" className="px-2 py-1 text-xs" onClick={() => remove(m)}>Del</Button>
                </div>
              </td>
            </tr>
          ))}
        </Table>
      )}
      <Modal title={editing ? 'Edit Menu' : 'New Menu'} open={open} onClose={() => setOpen(false)}>
        <form onSubmit={save} className="space-y-3">
          <Input label="Command" value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })} placeholder="/order" required />
          <Input label="Title (button label)" value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} placeholder="📦 List" />
          <Textarea label="Reply text" value={form.reply_text} onChange={(e) => setForm({ ...form, reply_text: e.target.value })} rows={3} />
          <Input label="Sort order" type="number" value={form.sort_order} onChange={(e) => setForm({ ...form, sort_order: e.target.value })} />
          {!editing && (
            <label className="flex items-center gap-2 font-bold">
              <input type="checkbox" checked={form.is_enabled} onChange={(e) => setForm({ ...form, is_enabled: e.target.checked })} /> Enabled
            </label>
          )}
          <div className="flex justify-end gap-2">
            <Button type="button" onClick={() => setOpen(false)}>Cancel</Button>
            <Button type="submit" variant="primary" disabled={saving}>{saving ? 'Saving…' : 'Save'}</Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
