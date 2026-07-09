import { useState, useEffect, useCallback } from 'react';
import { listAgents, createAgent, deleteAgent, cloneAgent, updateAgent } from '../../lib/api';
import type { AgentProfile } from '../../lib/types';
import { Plus, Trash2, Copy, Edit3, X, Check } from 'lucide-react';

export default function AgentManager() {
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [loading, setLoading] = useState(true);
  const [msg, setMsg] = useState('');
  const [editing, setEditing] = useState<string | null>(null);
  const [editPrompt, setEditPrompt] = useState('');
  const [newName, setNewName] = useState('');
  const [cloneTarget, setCloneTarget] = useState('');

  const load = useCallback(() => {
    setLoading(true);
    listAgents().then(a => setAgents(a.agents || [])).catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleCreate = async () => {
    if (!newName.trim()) { setMsg('❌ Name required'); return; }
    try {
      await createAgent(newName.trim());
      setMsg('✅ Created');
      setNewName('');
      setTimeout(() => setMsg(''), 2000);
      load();
    } catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  const handleDelete = async (name: string) => {
    if (!confirm(`Delete agent "${name}"?`)) return;
    try {
      await deleteAgent(name);
      setMsg('✅ Deleted');
      setTimeout(() => setMsg(''), 2000);
      load();
    } catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  const handleClone = async (sourceName: string) => {
    const name = cloneTarget || `${sourceName}-clone`;
    try {
      await cloneAgent(sourceName, name);
      setMsg(`✅ Cloned to "${name}"`);
      setCloneTarget('');
      setTimeout(() => setMsg(''), 2000);
      load();
    } catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  const startEdit = (agent: AgentProfile) => {
    setEditing(agent.name);
    setEditPrompt(agent.agent?.system_prompt || '');
  };

  const saveEdit = async (name: string) => {
    try {
      await updateAgent(name, { agent: { max_turns: 0, system_prompt: editPrompt } } as any);
      setMsg('✅ Updated');
      setEditing(null);
      setTimeout(() => setMsg(''), 2000);
      load();
    } catch (e: any) { setMsg(`❌ ${e.message}`); }
  };

  if (loading) return <div className="p-4 text-sm text-[#8b949e]">Loading agents...</div>;

  return (
    <div className="overflow-auto max-h-full">
      <div className="p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-[#e6edf3]">🤖 Agent Manager</h2>
          {msg && <span className={`text-xs ${msg.startsWith('✅') ? 'text-green-400' : 'text-red-400'}`}>{msg}</span>}
        </div>

        {/* Create new agent */}
        <div className="flex gap-2">
          <input
            value={newName}
            onChange={e => setNewName(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleCreate()}
            placeholder="New agent name (e.g. python-expert)"
            className="flex-1 bg-[#0d1117] border border-[#30363d] rounded px-3 py-2 text-xs text-[#e6edf3] font-mono"
          />
          <button onClick={handleCreate} className="px-3 py-2 bg-[#238636] text-white text-xs rounded hover:bg-[#2ea043] flex items-center gap-1">
            <Plus size={12} /> Create
          </button>
        </div>

        <div className="text-[10px] text-[#8b949e]">
          New agents are cloned from the built-in default. Edit them to add custom skills, MCP servers, and system prompts.
        </div>

        {/* Clone target name */}
        <div className="flex gap-2 items-center">
          <input
            value={cloneTarget}
            onChange={e => setCloneTarget(e.target.value)}
            placeholder="Clone as... (leave blank to auto-name)"
            className="flex-1 bg-[#0d1117] border border-[#30363d] rounded px-2 py-1.5 text-xs text-[#e6edf3] font-mono"
          />
        </div>

        {/* Agent list */}
        <div className="space-y-2">
          {agents.length === 0 ? (
            <div className="text-sm text-[#8b949e] py-4 text-center">No agents yet. Create one above.</div>
          ) : (
            agents.map(agent => (
              <div key={agent.name} className={`bg-[#0d1117] border ${agent.is_builtin ? 'border-[#30363d]' : 'border-[#238636]'} rounded-lg p-3`}>
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    <span className="text-lg">{agent.avatar || '🤖'}</span>
                    <div>
                      <span className="text-sm font-semibold text-[#e6edf3]">{agent.name}</span>
                      {agent.is_builtin && <span className="ml-2 text-[10px] bg-[#21262d] text-[#8b949e] px-1.5 py-0.5 rounded">built-in</span>}
                      {!agent.is_builtin && <span className="ml-2 text-[10px] bg-[#1a3a2a] text-[#3fb950] px-1.5 py-0.5 rounded">custom</span>}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    {!agent.is_builtin && (
                      <>
                        <button onClick={() => startEdit(agent)} className="p-1.5 text-[#8b949e] hover:text-[#e6edf3] rounded" title="Edit system prompt">
                          <Edit3 size={12} />
                        </button>
                        <button onClick={() => handleDelete(agent.name)} className="p-1.5 text-[#8b949e] hover:text-red-400 rounded" title="Delete agent">
                          <Trash2 size={12} />
                        </button>
                      </>
                    )}
                    <button onClick={() => handleClone(agent.name)} className="p-1.5 text-[#8b949e] hover:text-[#e6edf3] rounded" title="Clone agent">
                      <Copy size={12} />
                    </button>
                  </div>
                </div>

                <div className="text-xs text-[#8b949e] mb-1">{agent.description || 'No description'}</div>

                <div className="flex gap-2 flex-wrap text-[10px]">
                  <span className="bg-[#21262d] text-[#e6edf3] px-1.5 py-0.5 rounded">
                    {agent.model?.provider || 'openai'} / {agent.model?.model || 'gpt-4o'}
                  </span>
                  <span className="bg-[#21262d] text-[#e6edf3] px-1.5 py-0.5 rounded">
                    max {agent.agent?.max_turns || 60} turns
                  </span>
                  {agent.mcp?.servers && agent.mcp.servers.length > 0 && (
                    <span className="bg-[#21262d] text-[#f0883e] px-1.5 py-0.5 rounded">
                      {agent.mcp.servers.length} MCP
                    </span>
                  )}
                  {agent.skills?.dirs && agent.skills.dirs.length > 0 && (
                    <span className="bg-[#21262d] text-[#a371f7] px-1.5 py-0.5 rounded">
                      {agent.skills.dirs.length} skill dirs
                    </span>
                  )}
                </div>

                {/* Inline editor for system prompt */}
                {editing === agent.name && (
                  <div className="mt-3 border-t border-[#30363d] pt-3">
                    <label className="block text-[10px] text-[#8b949e] mb-1">System Prompt</label>
                    <textarea
                      value={editPrompt}
                      onChange={e => setEditPrompt(e.target.value)}
                      rows={4}
                      className="w-full bg-[#0d1117] border border-[#30363d] rounded px-3 py-2 text-xs text-[#e6edf3] font-mono resize-y mb-2"
                    />
                    <div className="flex gap-2">
                      <button onClick={() => saveEdit(agent.name)} className="px-2 py-1 bg-[#238636] text-white text-xs rounded flex items-center gap-1">
                        <Check size={10} /> Save
                      </button>
                      <button onClick={() => setEditing(null)} className="px-2 py-1 bg-[#21262d] text-[#8b949e] text-xs rounded flex items-center gap-1">
                        <X size={10} /> Cancel
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
