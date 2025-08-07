import { useEffect, useState } from 'react';

export default function Logs() {
    const [logs, setLogs] = useState<any[]>([]);
    const [error, setError] = useState<string | null>(null);
    const apiBase = import.meta.env.VITE_API_BASE_PATH;

    useEffect(() => {
        fetch(`${apiBase}/logs`)
            .then(async (res) => {
                const json = await res.json();
                if (! res.ok) {
                    throw new Error(json.error || `HTTP ${res.status}`);
                }
                return json;
            })
            .then(setLogs)
            .catch((err) => setError(err.message));
    }, []);

    if (error) {
        const message = error.includes('failed to read log file') ? (
            <>
                <p className="font-semibold">WAFログが読み取れません。</p>
                <p className="text-sm">
                    この機能を利用するには、環境変数
                    <code className="mx-1 px-1 py-0.5 bg-gray-200 rounded">WAF_LOG_FILE</code>
                    を設定し、有効なログファイルパスを指定してください。
                </p>
            </>
        ) : (
            `Error: ${error}`
        );

        return <div className="text-red-500 space-y-1">{message}</div>;
    }

    return (
        <div>
            <h2 className="text-2xl font-semibold mb-4">Logs</h2>
            {logs.length === 0 ? (
                <div className="text-gray-500">No logs available.</div>
            ) : (
                <ul className="space-y-2 text-sm bg-gray-100 p-4 rounded">
                    {logs.map((log, idx) => (
                        <li key={idx}>
                            <pre className="whitespace-pre-wrap">{JSON.stringify(log, null, 2)}</pre>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}
