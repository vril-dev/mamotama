import { useCallback, useEffect, useRef, useState } from "react";
import { apiGetJson, apiGetBinary } from "@/lib/api";

type LogSource = "waf" | "accerr" | "intr";

type LogLine = {
    ts?: string;
    req_id?: string;
    rule_id?: string | number;
    status?: number;
    path?: string;
    event?: string;
    method?: string;
    [k: string]: unknown;
};

type ReadResponse = {
    lines: LogLine[];
    next_cursor?: number;
    has_more: boolean;
    has_prev?: boolean;
    has_next?: boolean;
};

const SRC_TO_PARAM: Record<LogSource, string> = {
    waf: "waf",
    accerr: "accerr",
    intr: "intr",
};

async function readLogs(params: {
    src: LogSource;
    tail: number;
    cursor?: number;
    dir?: "prev" | "next";
    signal?: AbortSignal;
}): Promise<ReadResponse> {
    const q = new URLSearchParams();
    q.set("src", SRC_TO_PARAM[params.src]);
    q.set("tail", String(params.tail));

    if (params.cursor != null) {
        q.set("cursor", String(params.cursor));
    }

    if (params.dir) {
        q.set("dir", params.dir);
    }

    return apiGetJson<ReadResponse>(`/logs/read?${q.toString()}`, {
        signal: params.signal,
    });
}

function buildDownloadPath(src: LogSource) {
    const q = new URLSearchParams();
    q.set("src", SRC_TO_PARAM[src]);

    return `/logs/download?${q.toString()}`;
}

function classNames(...xs: (string | false | null | undefined)[]) {
    return xs.filter(Boolean).join(" ");
}

function colorClass(line: LogLine) {
    if (line.event === "waf_block") {
        return "bg-red-50 text-red-700";
    }

    if (line.event === "waf_hit_allow") {
        return "bg-yellow-50 text-yellow-700";
    }

    if (typeof line.status === "number" && line.status >= 500) {
        return "bg-orange-50 text-orange-700";
    }

    return "";
}

export default function Logs() {
    const [src, setSrc] = useState<LogSource>("waf");
    const [tail, setTail] = useState<number>(30);
    const [pageStart, setPageStart] = useState<number | undefined>(undefined);
    const [pageEnd, setPageEnd] = useState<number | undefined>(undefined);
    const [dir, setDir] = useState<"prev" | "next" | undefined>(undefined);
    const [cursor, setCursor] = useState<number | undefined>(undefined);
    const [data, setData] = useState<ReadResponse | null>(null);
    const [loading, setLoading] = useState(false);
    const [canPrev, setCanPrev] = useState(false);
    const [canNext, setCanNext] = useState(false);

    const controllerRef = useRef<AbortController | null>(null);

    const load = useCallback(
        async (opts?: {
            reset?: boolean;
            dir?: "prev" | "next";
            cursor?: number;
        }) => {
            if (controllerRef.current) {
                controllerRef.current.abort("refresh");
            }

            const ac = new AbortController();
            controllerRef.current = ac;

            setLoading(true);
            try {
                const useDir = opts?.dir ?? dir;

                let useCursor: number | undefined;
                if (opts?.reset || useDir === undefined) {
                    useCursor = undefined;
                } else if (opts?.cursor != null) {
                    useCursor = opts.cursor;
                } else if (useDir === "prev") {
                    useCursor = pageStart;
                } else if (useDir === "next") {
                    useCursor = pageEnd;
                }

                const r = await readLogs({
                    src,
                    tail,
                    cursor: useCursor,
                    dir: useDir,
                    signal: ac.signal,
                });

                setData(r);

                const n = r?.lines?.length ?? 0;
                if (useDir === "prev") {
                    const start = r.next_cursor ?? 0;
                    const end = start + n;
                    setPageStart(start);
                    setPageEnd(end);
                    setCursor(start);
                } else if (useDir === "next") {
                    const end = r.next_cursor ?? 0;
                    const start = Math.max(0, end - n);
                    setPageStart(start);
                    setPageEnd(end);
                    setCursor(end);
                } else {
                    const end = r.next_cursor ?? 0;
                    const start = Math.max(0, end - n);
                    setPageStart(start);
                    setPageEnd(end);
                    setCursor(end);
                }
                setDir(useDir);

                if (typeof r.has_prev === "boolean" || typeof r.has_next === "boolean") {
                    setCanNext(!!r.has_prev);
                    setCanPrev(!!r.has_next);
                } else {
                    if (opts?.reset || useDir === undefined) {
                        setCanPrev(false);
                        setCanNext(!!r.has_more);
                    } else if (useDir === "next") {
                        setCanPrev(true);
                        setCanNext(!!r.has_more);
                    } else if (useDir === "prev") {
                        setCanPrev(!!r.has_more);
                        setCanNext(true);
                    }
                }
            } catch (e: any) {
                const isAbort =
                    e?.name === "AbortError" ||
                    e?.code === 20 ||
                    e === "refresh" ||
                    e === "unmount";
                if (isAbort) {
                    return;
                }

                console.error(e);
            } finally {
                setLoading(false);
            }
        },
        [pageStart, pageEnd, dir, src, tail]
    );

    useEffect(() => {
        load({ reset: true });
    }, [src, tail]);

    useEffect(() => {
        return () => {
            if (controllerRef.current) {
                controllerRef.current.abort("unmount");
            }
        };
    }, []);

    async function downloadAll() {
        const sources: LogSource[] = ["waf", "accerr", "intr"];
        for (const s of sources) {
            const path = buildDownloadPath(s);
            const { blob, filename } = await apiGetBinary(path);
            const a = document.createElement("a");
            const url = URL.createObjectURL(blob);
            a.href = url;
            a.download =
                filename ||
                `${s}-${new Date().toISOString().slice(0, 10).replace(/-/g, "")}.ndjson.gz`;
            document.body.appendChild(a);
            a.click();
            a.remove();
            URL.revokeObjectURL(url);
        }
    }

    return (
        <div className="p-4 space-y-4">
            <h1 className="text-xl font-semibold">Logs</h1>

            <div className="flex flex-wrap items-center gap-2">
                <div className="inline-flex rounded-lg border overflow-hidden">
                {(["waf", "accerr", "intr"] as LogSource[]).map((s) => (
                    <button
                        key={s}
                        onClick={() => {
                            setSrc(s);
                            setCursor(undefined);
                            setDir(undefined);
                        }}
                        className={classNames(
                            "px-3 py-1 text-sm",
                            src === s ? "bg-black text-white" : "bg-white"
                        )}
                    >
                        {s === "waf" ? "waf-events" : s === "accerr" ? "access-error" : "interesting"}
                    </button>
                ))}
                </div>

                <label className="ml-2 text-sm">
                    Rows:
                    <select
                        className="ml-1 border rounded px-1 py-1"
                        value={tail}
                        onChange={(e) => setTail(Number(e.target.value))}
                    >
                        {[30, 50, 100, 200].map((n) => (
                        <option key={n} value={n}>
                            {n}
                        </option>
                        ))}
                    </select>
                </label>

                <button
                    className="ml-auto underline text-sm"
                    onClick={async () => {
                        try {
                            await downloadAll();
                        } catch (e) {
                            console.error(e);
                            alert("ダウンロードに失敗しました。コンソールを確認してください。");
                        }
                    }}
                    title="3ファイルをダウンロード（期間指定なし）"
                >
                    Download
                </button>

                <button
                    disabled={loading}
                    onClick={() => {
                        setDir(undefined);
                        setCursor(undefined);
                        load({ reset: true });
                    }}
                    className="border rounded px-3 py-1 text-sm"
                    title="末尾から読み直し"
                >
                    Reload latest
                </button>
            </div>

            <div className="flex items-center gap-2">
                <button
                    disabled={loading || !canPrev}
                    onClick={() => load({ dir: "next" })}
                    className="border rounded px-3 py-1 text-sm"
                    title="前へ"
                >
                    ◀ prev
                </button>
                <button
                    disabled={loading || !canNext}
                    onClick={() => load({ dir: "prev" })}
                    className="border rounded px-3 py-1 text-sm"
                    title="次へ"
                >
                    next ▶
                </button>
                {loading && <span className="text-sm text-gray-500">loading…</span>}
            </div>

            <div className="border rounded overflow-hidden">
                <table className="min-w-full text-sm">
                    <thead className="bg-gray-50">
                        <tr>
                            <th className="px-2 py-1 text-left">ts</th>
                            <th className="px-2 py-1 text-left">status</th>
                            <th className="px-2 py-1 text-left">event</th>
                            <th className="px-2 py-1 text-left">rule_id</th>
                            <th className="px-2 py-1 text-left">method</th>
                            <th className="px-2 py-1 text-left">path</th>
                            <th className="px-2 py-1 text-left">req_id</th>
                            <th className="px-2 py-1 text-left">raw</th>
                        </tr>
                    </thead>
                    <tbody>
                    {data?.lines?.map((line, i) => (
                        <tr key={i} className={classNames(colorClass(line))}>
                            <td className="px-2 py-1 whitespace-nowrap">
                                {line.ts ? new Date(line.ts).toLocaleString() : "-"}
                            </td>
                            <td className="px-2 py-1">{line.status ?? "-"}</td>
                            <td className="px-2 py-1">{String(line.event ?? "-")}</td>
                            <td className="px-2 py-1">{String(line.rule_id ?? "-")}</td>
                            <td className="px-2 py-1">{line.method ?? "-"}</td>
                            <td className="px-2 py-1">{line.path ?? "-"}</td>
                            <td className="px-2 py-1">
                                {line.req_id ? (
                                <code className="cursor-pointer" title="クリックでコピー"
                                    onClick={() => navigator.clipboard.writeText(String(line.req_id))}
                                >
                                    {String(line.req_id)}
                                </code>
                                ) : "-"}
                            </td>
                            <td className="px-2 py-1">
                                <details>
                                    <summary className="cursor-pointer underline">JSON</summary>
                                    <pre className="p-2 bg-gray-50 rounded overflow-x-auto">
                                        {JSON.stringify(line, null, 2)}
                                    </pre>
                                </details>
                            </td>
                        </tr>
                    ))}
                    {!data?.lines?.length && (
                        <tr>
                        <td colSpan={8} className="px-2 py-6 text-center text-gray-500">
                            No data.
                        </td>
                        </tr>
                    )}
                    </tbody>
                </table>
            </div>

            {data && (
                <div className="text-xs text-gray-500">
                    has_more: {String(data.has_more)} / next_cursor:{" "}
                    {data.next_cursor != null ? data.next_cursor : "-"}
                    {" "} / page: [{pageStart ?? "-"}, {pageEnd ?? "-"})
                </div>
            )}
        </div>
    );
}
