import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { apiGetJson, apiPostJson, apiPutJson } from "@/lib/api";

type RuleItem = {
    name: string;
    path: string;
    enabled: boolean;
};

type RuleSetsResp = {
    crs_enabled?: boolean;
    disabled_file?: string;
    etag?: string;
    rules?: RuleItem[];
};

export default function RuleSets() {
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [etag, setEtag] = useState<string | null>(null);
    const [disabledFile, setDisabledFile] = useState<string>("");
    const [crsEnabled, setCrsEnabled] = useState(true);
    const [rules, setRules] = useState<RuleItem[]>([]);
    const [serverEnabled, setServerEnabled] = useState<Set<string>>(new Set());
    const [valid, setValid] = useState<boolean | null>(null);
    const [messages, setMessages] = useState<string[]>([]);
    const [lastSavedAt, setLastSavedAt] = useState<number | null>(null);

    const load = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const data = await apiGetJson<RuleSetsResp>("/crs-rule-sets");
            const list = Array.isArray(data.rules) ? data.rules : [];
            setRules(list);
            setCrsEnabled(data.crs_enabled !== false);
            setDisabledFile(data.disabled_file ?? "");
            setEtag(data.etag ?? null);

            const enabled = new Set<string>();
            for (const r of list) {
                if (r.enabled) {
                    enabled.add(r.name);
                }
            }
            setServerEnabled(enabled);
            setValid(null);
            setMessages([]);
        } catch (e: any) {
            setError(e?.message || String(e));
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        void load();
    }, [load]);

    const enabledNames = useMemo(
        () => rules.filter((r) => r.enabled).map((r) => r.name).sort(),
        [rules]
    );
    const dirty = useMemo(() => {
        if (enabledNames.length !== serverEnabled.size) {
            return true;
        }
        for (const n of enabledNames) {
            if (!serverEnabled.has(n)) {
                return true;
            }
        }
        return false;
    }, [enabledNames, serverEnabled]);

    const debounceRef = useRef<number | null>(null);
    useEffect(() => {
        if (loading || !dirty) {
            return;
        }
        if (debounceRef.current) {
            window.clearTimeout(debounceRef.current);
        }

        debounceRef.current = window.setTimeout(async () => {
            try {
                const js = await apiPostJson<{ ok: boolean; messages?: string[] }>(
                    "/crs-rule-sets:validate",
                    { enabled: enabledNames }
                );
                setValid(!!js.ok);
                setMessages(Array.isArray(js.messages) ? js.messages : []);
            } catch (e: any) {
                setValid(false);
                setMessages([e?.message || "Validate failed"]);
            }
        }, 300);

        return () => {
            if (debounceRef.current) {
                window.clearTimeout(debounceRef.current);
            }
        };
    }, [enabledNames, dirty, loading]);

    const toggle = useCallback((name: string, on: boolean) => {
        setRules((prev) => prev.map((r) => (r.name === name ? { ...r, enabled: on } : r)));
    }, []);

    const setAll = useCallback((on: boolean) => {
        setRules((prev) => prev.map((r) => ({ ...r, enabled: on })));
    }, []);

    const doSave = useCallback(async () => {
        setSaving(true);
        setError(null);
        try {
            const js = await apiPutJson<{ ok: boolean; etag?: string }>(
                "/crs-rule-sets",
                { enabled: enabledNames },
                { headers: etag ? { "If-Match": etag } : {} }
            );
            if (!js.ok) {
                throw new Error("保存に失敗しました");
            }
            setEtag(js.etag ?? null);
            setServerEnabled(new Set(enabledNames));
            setLastSavedAt(Date.now());
        } catch (e: any) {
            setError(e?.message || "Save failed");
        } finally {
            setSaving(false);
        }
    }, [enabledNames, etag]);

    if (loading) {
        return <div className="text-gray-500">Loading rule sets...</div>;
    }

    return (
        <div className="w-full max-w-5xl mx-auto p-4 space-y-4">
            <header className="flex items-center justify-between">
                <h1 className="text-xl font-semibold">Rule Sets</h1>
                <div className="flex items-center gap-2">
                    {!crsEnabled && <Badge color="amber">CRS Disabled</Badge>}
                    {valid === null ? <Badge color="gray">Unvalidated</Badge> : valid ? <Badge color="green">Valid</Badge> : <Badge color="red">Invalid</Badge>}
                    {dirty && <Badge color="amber">Unsaved</Badge>}
                    {etag && <MonoTag label="ETag" value={etag} />}
                </div>
            </header>

            {error && <div className="border border-red-300 bg-red-50 rounded-xl p-3 text-sm">エラー: {error}</div>}

            <div className="text-sm text-neutral-600">
                CRS本体ルールの有効/無効を切り替えます。保存後にWAFをホットリロードします。
                {disabledFile && <span className="ml-2 text-neutral-500">保存先: <code>{disabledFile}</code></span>}
            </div>

            <div className="flex items-center gap-2">
                <button
                    type="button"
                    className="px-3 py-1.5 rounded-xl shadow text-sm hover:bg-neutral-50 border"
                    onClick={() => setAll(true)}
                    disabled={saving}
                >
                    全て有効
                </button>
                <button
                    type="button"
                    className="px-3 py-1.5 rounded-xl shadow text-sm hover:bg-neutral-50 border"
                    onClick={() => setAll(false)}
                    disabled={saving}
                >
                    全て無効
                </button>
                <button
                    type="button"
                    className="px-3 py-1.5 rounded-xl shadow text-sm hover:bg-neutral-50 border"
                    onClick={() => void load()}
                    disabled={saving}
                >
                    最新を取得
                </button>
                <button
                    type="button"
                    className="px-3 py-1.5 rounded-xl shadow text-sm bg-black text-white disabled:opacity-50"
                    onClick={() => void doSave()}
                    disabled={saving || !dirty}
                >
                    {saving ? "保存中…" : "保存してホットリロード"}
                </button>
            </div>

            <div className="border rounded-xl overflow-hidden bg-white">
                <div className="grid grid-cols-[100px_1fr_1fr] gap-0 text-xs font-semibold bg-neutral-100 border-b">
                    <div className="p-2">Enabled</div>
                    <div className="p-2">File</div>
                    <div className="p-2">Path</div>
                </div>
                <div className="max-h-[520px] overflow-auto">
                    {rules.map((r) => (
                        <label key={r.path} className="grid grid-cols-[100px_1fr_1fr] border-b last:border-b-0 text-sm items-center">
                            <div className="p-2">
                                <input
                                    type="checkbox"
                                    checked={r.enabled}
                                    onChange={(e) => toggle(r.name, e.target.checked)}
                                    disabled={saving}
                                />
                            </div>
                            <div className="p-2 font-mono">{r.name}</div>
                            <div className="p-2 font-mono text-xs text-neutral-600">{r.path}</div>
                        </label>
                    ))}
                    {rules.length === 0 && (
                        <div className="p-4 text-sm text-neutral-500">CRSルールファイルが見つかりません。</div>
                    )}
                </div>
            </div>

            <div className="flex items-center justify-between text-xs text-neutral-500">
                <div className="flex items-center gap-3">
                    <span>合計: {rules.length}</span>
                    <span>有効: {enabledNames.length}</span>
                    <span>無効: {rules.length - enabledNames.length}</span>
                    {lastSavedAt && <span>最終保存: {new Date(lastSavedAt).toLocaleString()}</span>}
                </div>
                <div className="flex items-center gap-2">
                    {messages.slice(0, 3).map((m, i) => (
                        <span key={i} className="px-2 py-0.5 bg-neutral-100 rounded">{m}</span>
                    ))}
                </div>
            </div>
        </div>
    );
}

function Badge({ color, children }: { color: "gray" | "green" | "red" | "amber"; children: React.ReactNode }) {
    const cls =
        color === "green" ? "bg-green-100 text-green-800" :
        color === "red" ? "bg-red-100 text-red-800" :
        color === "amber" ? "bg-amber-100 text-amber-800" :
        "bg-neutral-100 text-neutral-700";

    return <span className={`px-2 py-0.5 text-xs rounded ${cls}`}>{children}</span>;
}

function MonoTag({ label, value }: { label: string; value: string }) {
    return (
        <div className="hidden md:flex items-center gap-1 text-xs">
            <span className="text-neutral-500">{label}:</span>
            <code className="px-2 py-0.5 bg-neutral-100 rounded max-w-[420px] truncate">{value}</code>
        </div>
    );
}
