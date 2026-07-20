import { useEffect, useState } from 'react';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { Button, Card, Spinner, EmptyState } from '../components/ui.jsx';

export default function Settings() {
  const toast = useToast();
  const [items, setItems] = useState(null);
  const [drafts, setDrafts] = useState({});
  const [savingKey, setSavingKey] = useState(null);

  const load = () => {
    setItems(null);
    api.listSettings().then((d) => {
      const list = d.items || [];
      setItems(list);
      setDrafts(Object.fromEntries(list.map((s) => [s.key, s.value])));
    }).catch((e) => toast.error(e.message));
  };
  useEffect(load, []);

  const save = async (key) => {
    setSavingKey(key);
    try { await api.setSetting(key, drafts[key] ?? ''); toast.success(`Saved ${key}`); }
    catch (e) { toast.error(e.message); }
    finally { setSavingKey(null); }
  };

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">Settings</h1>
      {items === null ? <Spinner /> : items.length === 0 ? <EmptyState>No settings</EmptyState> : (
        <div className="space-y-3">
          {items.map((s) => (
            <Card key={s.key} className="bg-white">
              <div className="mb-2 font-mono font-bold">{s.key}</div>
              <div className="flex gap-2">
                <input
                  className="nb-input flex-1"
                  value={drafts[s.key] ?? ''}
                  onChange={(e) => setDrafts({ ...drafts, [s.key]: e.target.value })}
                />
                <Button variant="primary" onClick={() => save(s.key)} disabled={savingKey === s.key}>
                  {savingKey === s.key ? 'Saving…' : 'Save'}
                </Button>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
