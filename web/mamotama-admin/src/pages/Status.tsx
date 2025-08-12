import { useEffect, useState } from 'react';
import { apiGetJson } from "@/lib/api";

export default function State() {
    const [data, setData] = useState<any>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        apiGetJson("/status")
            .then(setData)
            .catch((err) => setError(err.mesasge));
    }, []);

    if (error) {
        return <div className="text-red-500">Error: {error}</div>;
    }

    if (!data) {
        return <div className="text-gray-500">Loading status...</div>;
    }

    return (
        <div>
            <h2 className="text-2xl font-semiboild mb-4">WAF Status</h2>
            <pre className="bg-gray-100 text-sm rounded p-4 shadow">
                {JSON.stringify(data, null, 2)}
            </pre>
        </div>
    );
}
