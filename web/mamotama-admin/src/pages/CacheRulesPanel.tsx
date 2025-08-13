import { useEffect, useState } from "react";
import { apiGetJson, apiPostJson, apiPutJson } from "@/lib/api";

type Match = { type: "prefix" | "regex" | "exact"; value: string };
type Rule = {
    kind: "ALLOW" | "DENY";
    match: Match;
    methods?: string[];
    ttl?: number;
    vary?: string[];
};
type RulesDTO = {
    etag: string;
    raw: string;
    rules: Rule[];
    errors?: string[];
};

export default function CacheRulePanel() {
    const [rules, setRules] = useState<Rule[]>([]);
    const [raw, setRaw] = useState("");
    const [rawMode, setRawMode] = useState(false);
    const [etag, setEtag] = useState("");
    const [loading, setLoading] = useState(false);
    const [msgs, setMsgs] = useState<string[]>([]);

    useEffect(() => {
        void reload(); 
    }, []);

    async function reload() {
        setLoading(true);
        setMsgs([]);

        try {
            const { etag, rules, raw } = await apiGetJson<RulesDTO>("/cache-rules");
            setRules(rules ?? []);
            setRaw(raw ?? "");
            setEtag(etag ?? "");
        } catch (e: any) {
            setMsgs([e?.message || "ロードに失敗"]);
        } finally {
            setLoading(false);
        }
    }

    function update(i: number, v: Rule) {
        setRules(rs => rs.map((r, idx) => (idx === i ? v : r)));
    }

    function remove(i: number) {
        setRules(rs => rs.filter((_, idx) => idx !== i));
    }

    function addRule() {
        setRules(r => [
            ...r,
            {
                kind: "ALLOW",
                match: {
                    type: "prefix",
                    value: "/"
                },
                methods: [
                    "GET", "HEAD"
                ],
                ttl: 600,
                vary: ["Accept-Encoding"]
            },
        ]);
    }

    async function validate() {
        setMsgs([]);
        const body = rawMode ? { rawMode: true, raw } : { rawMode: false, rules };

        try {
            const js = await apiPostJson<{ ok: Boolean; messages?: string[] }>("/cache-rules:validate", body);
            if (! js.ok) {
                setMsgs(js.messages ?? ["validation error"]);
            } else {
                setMsgs(["OK"]);
            }
        } catch (e: any) {
            setMsgs([e?.message || "validate failed"]);
        }
    }

    async function save() {
        setMsgs([]);
        const body = rawMode ? { rawMode: true, raw } : { rawMode: false, rules };

        try {
            const js = await apiPutJson<{ ok: Boolean; etag?: string }>("/cache-rules", body, {
                headers: {
                    "If-Match": etag
                },
            });

            if (! js.ok) {
                throw new Error("save failed");
            }

            setEtag(js.etag ?? "");
            setMsgs(["保存しました。ホットリロードで即時反映されます。"]);
        } catch (e: any) {
            setMsgs([e?.message || "save failed"]);
        }
    }

    return (
        <div className="p-4 space-y-3">
            <div className="flex items-center gap-3">
                <h2 className="text-lg font-semibold">Cache Rules</h2>
                <label className="flex items-center gap-1">
                <input type="checkbox" checked={rawMode} onChange={e => setRawMode(e.target.checked)} />
                Raw 編集
                </label>
                <button onClick={reload} disabled={loading} className="px-3 py-1 border rounded">Reload</button>
                <button onClick={validate} disabled={loading} className="px-3 py-1 border rounded">Validate</button>
                <button onClick={save} disabled={loading} className="px-3 py-1 border rounded">Save</button>
            </div>

            {! rawMode ? (
                <div className="space-y-2">
                <button onClick={addRule} className="px-2 py-1 border rounded">ルール追加</button>
                <div className="overflow-x-auto">
                    <table className="min-w-[800px] w-full text-sm border">
                    <thead>
                        <tr className="bg-neutral-900/20">
                        <th className="p-2 border">Kind</th>
                        <th className="p-2 border">Match Type</th>
                        <th className="p-2 border">Value</th>
                        <th className="p-2 border">Methods</th>
                        <th className="p-2 border">TTL(s)</th>
                        <th className="p-2 border">Vary</th>
                        <th className="p-2 border w-24"></th>
                        </tr>
                    </thead>
                    <tbody>
                        {rules.map((r, i) => (
                        <tr key={i}>
                            <td className="p-1 border">
                            <select value={r.kind} onChange={e => update(i, { ...r, kind: e.target.value as any })} className="w-full">
                                <option>ALLOW</option>
                                <option>DENY</option>
                            </select>
                            </td>
                            <td className="p-1 border">
                            <select
                                value={r.match.type}
                                onChange={e => update(i, { ...r, match: { ...r.match, type: e.target.value as any } })}
                                className="w-full"
                            >
                                <option>prefix</option>
                                <option>regex</option>
                                <option>exact</option>
                            </select>
                            </td>
                            <td className="p-1 border">
                            <input className="w-full" value={r.match.value} onChange={e => update(i, { ...r, match: { ...r.match, value: e.target.value } })} />
                            </td>
                            <td className="p-1 border">
                            <input
                                className="w-full"
                                value={(r.methods ?? []).join(",")}
                                onChange={e => update(i, { ...r, methods: e.target.value.split(",").map(s => s.trim()).filter(Boolean) })}
                                placeholder="GET,HEAD"
                            />
                            </td>
                            <td className="p-1 border">
                            <input
                                type="number"
                                className="w-full"
                                value={r.ttl ?? 0}
                                onChange={e => update(i, { ...r, ttl: Number.isFinite(+e.target.value) ? parseInt(e.target.value || "0", 10) : 0 })}
                            />
                            </td>
                            <td className="p-1 border">
                            <input
                                className="w-full"
                                value={(r.vary ?? []).join(",")}
                                onChange={e => update(i, { ...r, vary: e.target.value.split(",").map(s => s.trim()).filter(Boolean) })}
                                placeholder="Accept-Encoding,Accept-Language"
                            />
                            </td>
                            <td className="p-1 border text-center">
                            <button onClick={() => remove(i)} className="px-2 py-1 border rounded">削除</button>
                            </td>
                        </tr>
                        ))}
                        {rules.length === 0 && (
                        <tr>
                            <td colSpan={7} className="p-3 text-center text-neutral-400">ルールがありません</td>
                        </tr>
                        )}
                    </tbody>
                    </table>
                </div>
                </div>
            ) : (
                <textarea className="w-full h-96 font-mono p-2 border rounded" value={raw} onChange={e => setRaw(e.target.value)} />
            )}

            {msgs.length > 0 && (
                <div className="border p-2">
                {msgs.map((m, i) => (<div key={i}>{m}</div>))}
                </div>
            )}
        </div>
    );
}

