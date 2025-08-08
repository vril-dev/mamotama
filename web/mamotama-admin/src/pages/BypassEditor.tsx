import { useEffect, useState } from "react";

export default function BypassEditor() {
    const [text, setText] = useState("");
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState("");
    const [success, setSuccess] = useState(false);
    const apiBase = import.meta.env.VITE_API_BASE_PATH;

    useEffect(() => {
        fetch(`${apiBase}/bypass`)
            .then((res) => {
                if (! res.ok) {
                    throw new Error("Failed to fetch bypass config");
                }
                return res.text();
            })
            .then(setText)
            .catch((err) => setError(err.message))
            .finally(() => setLoading(false));
    }, []);

    const handleSave = async () => {
        setSaving(true);
        setError("");
        setSuccess(false);

        try {
            const res = await fetch(`${apiBase}/bypass`, {
                method: "POST",
                headers: { "Content-Type": "text/plain" },
                body: text,
            });

            if (! res.ok) {
                throw new Error("Failed to save bypass config");
            }
            setSuccess(true);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Unknown error");
        } finally {
            setSaving(false);
        }
    };

    if (loading) {
        return <p>Loading bypass config...</p>;
    }

    if (error) {
        return <p className="text-red-500">{error}</p>;
    }

    return (
        <div className="p-4">
            <h2 className="text-2xl font-semibold mb-4">Bypass Configuration</h2>
            <textarea
                className="w-full h-96 p-2 border rounded font-mono"
                value={text}
                onChange={(e) => setText(e.target.value)}
            />
            <div className="mt-4 flex items-center space-x-4">
                <button
                    className="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded"
                    onClick={handleSave}
                    disabled={saving}
                >{saving ? "Saving..." : "Save"}</button>
                {success && <span className="text-green-600">Saved!</span>}
            </div>
        </div>
    );
}
