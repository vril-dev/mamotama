import { useEffect, useMemo, useState } from 'react';
import { apiGetJson } from "@/lib/api";

type StatusResponse = Record<string, unknown>;

export default function Status() {
    const [data, setData] = useState<StatusResponse | null>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        apiGetJson("/status")
            .then(setData)
            .catch((err: unknown) => {
                const message = err instanceof Error ? err.message : String(err);
                setError(message);
            });
    }, []);

    const running = useMemo(() => (data?.status === "running"), [data]);

    if (error) {
        return (
            <div className="w-full p-4">
                <div className="border border-red-300 bg-red-50 rounded-xl p-3 text-sm">Error: {error}</div>
            </div>
        );
    }

    if (!data) {
        return <div className="w-full p-4 text-gray-500">Loading status...</div>;
    }

    return (
        <div className="w-full p-4 space-y-4">
            <header className="flex items-center justify-between">
                <h1 className="text-xl font-semibold">Status</h1>
                <span
                    className={`px-2 py-0.5 text-xs rounded ${
                        running ? "bg-green-100 text-green-800" : "bg-amber-100 text-amber-800"
                    }`}
                >
                    {running ? "Running" : "Degraded"}
                </span>
            </header>

            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4 text-sm">
                <Metric label="API Base" value={String(data.api_base ?? "-")} />
                <Metric label="Rules File" value={String(data.rules_file ?? "-")} />
                <Metric label="CRS Enabled" value={String(data.crs_enabled ?? "-")} />
                <Metric label="Rate Limit Enabled" value={String(data.rate_limit_enabled ?? "-")} />
                <Metric label="Bot Defense Enabled" value={String(data.bot_defense_enabled ?? "-")} />
                <Metric label="Semantic Mode" value={String(data.semantic_mode ?? "-")} />
            </div>

            <pre className="text-sm rounded-xl p-4 shadow-sm overflow-x-auto">
                {JSON.stringify(data, null, 2)}
            </pre>
        </div>
    );
}

function Metric({ label, value }: { label: string; value: string }) {
    return (
        <div className="rounded-xl border bg-white px-3 py-2">
            <div className="text-xs text-neutral-500">{label}</div>
            <div className="font-mono text-xs mt-1 break-all">{value}</div>
        </div>
    );
}
