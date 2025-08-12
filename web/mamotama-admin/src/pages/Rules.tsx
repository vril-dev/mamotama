import { useEffect, useState } from 'react';

type RuleMap = Record<string, string>;

export default function Rules() {
    const [rules, setRules] = useState<RuleMap | null>(null);
    const [error, setError] = useState<string | null>(null);
    const apiBase = import.meta.env.VITE_API_BASE_PATH;

    useEffect(() => {
        fetch(`${apiBase}/rules`)
            .then(async (res) => {
                const json = await res.json();
                if (! res.ok) {
                    throw new Error(json.error || `HTTP ${res.status}`);
                }
                return json;
            })
            .then((data) => setRules(data.rules))
            .catch((err) => setError(err.message));
    }, []);

    if (error) {
        return <div className="text-red-500">Error : {error}</div>
    }

    if (! rules) {
        return <div className="text-gray-500">Loading rules...</div>
    }

    return (
        <div>
            <h2 className="text-2xl font-smibold mb-4">WAFルール一覧</h2>
            <div className="space-y-6">
                {Object.entries(rules).map(([filename, content]) => (
                    <div key={filename}>
                        <h3 className="font-bold text-lg mb-1">{filename}</h3>
                        <pre className="bg-gray-100 text-sm p-4 rounded whitespace-pre-wrap">
                            {content}
                        </pre>
                    </div>
                ))}
            </div>
        </div>
    );
}
