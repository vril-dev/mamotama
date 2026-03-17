import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { apiPostJson } from "@/lib/api";

type FPTunerProposal = {
  id: string;
  title?: string;
  summary?: string;
  reason?: string;
  confidence?: number;
  target_path: string;
  rule_line: string;
};

type ProposeResponse = {
  ok?: boolean;
  contract_version?: string;
  mode?: string;
  source?: string;
  input?: Record<string, unknown>;
  approval?: {
    required?: boolean;
    token?: string;
  };
  proposal?: FPTunerProposal;
};

type ApplyResponse = {
  ok?: boolean;
  contract_version?: string;
  simulated?: boolean;
  duplicate?: boolean;
  hot_reloaded?: boolean;
  reloaded_file?: string;
  etag?: string;
  preview_etag?: string;
};

type EventInput = {
  event_id: string;
  method: string;
  path: string;
  rule_id: string;
  status: string;
  matched_variable: string;
  matched_value: string;
};

const defaultEvent: EventInput = {
  event_id: "manual-ui-001",
  method: "GET",
  path: "/search",
  rule_id: "100004",
  status: "403",
  matched_variable: "ARGS:q",
  matched_value: "select * from users",
};

const defaultTargetPath = "rules/mamotama.conf";

export default function FPTunerPanel() {
  const [targetPath, setTargetPath] = useState(defaultTargetPath);
  const [useLatestEvent, setUseLatestEvent] = useState(false);
  const [eventInput, setEventInput] = useState<EventInput>(defaultEvent);

  const [proposal, setProposal] = useState<FPTunerProposal | null>(null);
  const [approvalRequired, setApprovalRequired] = useState(false);
  const [approvalToken, setApprovalToken] = useState("");
  const [simulate, setSimulate] = useState(true);

  const [mode, setMode] = useState<string>("-");
  const [source, setSource] = useState<string>("-");
  const [contractVersion, setContractVersion] = useState<string>("-");

  const [proposeResult, setProposeResult] = useState<ProposeResponse | null>(null);
  const [applyResult, setApplyResult] = useState<ApplyResponse | null>(null);

  const [proposing, setProposing] = useState(false);
  const [applying, setApplying] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canApply = useMemo(() => proposal && proposal.rule_line.trim() !== "", [proposal]);

  function updateEvent<K extends keyof EventInput>(key: K, value: EventInput[K]) {
    setEventInput((prev) => ({ ...prev, [key]: value }));
  }

  function updateProposal<K extends keyof FPTunerProposal>(key: K, value: FPTunerProposal[K]) {
    setProposal((prev) => {
      if (!prev) return prev;
      return { ...prev, [key]: value };
    });
  }

  async function onPropose() {
    setError(null);
    setApplyResult(null);
    setProposing(true);
    try {
      const payload: Record<string, unknown> = {
        target_path: targetPath.trim(),
      };

      if (!useLatestEvent) {
        payload.event = {
          event_id: eventInput.event_id.trim(),
          method: eventInput.method.trim() || "GET",
          path: eventInput.path.trim() || "/",
          rule_id: parseInt(eventInput.rule_id, 10) || 100004,
          status: parseInt(eventInput.status, 10) || 403,
          matched_variable: eventInput.matched_variable.trim() || "ARGS:q",
          matched_value: eventInput.matched_value,
        };
      }

      const res = await apiPostJson<ProposeResponse>("/fp-tuner/propose", payload);
      setProposeResult(res);
      setProposal(res.proposal ?? null);
      setApprovalRequired(!!res.approval?.required);
      setApprovalToken(res.approval?.token ?? "");
      setMode(res.mode || "-");
      setSource(res.source || "-");
      setContractVersion(res.contract_version || "-");
    } catch (e: any) {
      setError(e?.message || "Propose failed");
    } finally {
      setProposing(false);
    }
  }

  async function onApply() {
    if (!proposal) return;
    setError(null);
    setApplying(true);
    try {
      const payload = {
        proposal,
        simulate,
        approval_token: approvalToken,
      };
      const res = await apiPostJson<ApplyResponse>("/fp-tuner/apply", payload);
      setApplyResult(res);
    } catch (e: any) {
      setError(e?.message || "Apply failed");
    } finally {
      setApplying(false);
    }
  }

  return (
    <div className="w-full p-4 space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h1 className="text-xl font-semibold">FP Tuner</h1>
        <div className="flex items-center gap-2 text-xs">
          <Badge color="gray">mode: {mode}</Badge>
          <Badge color="gray">source: {source}</Badge>
          <Badge color="gray">contract: {contractVersion}</Badge>
          {approvalRequired ? <Badge color="amber">approval required</Badge> : <Badge color="green">approval optional</Badge>}
        </div>
      </header>

      {error && (
        <div className="border border-red-300 bg-red-50 rounded-xl p-3 text-sm">
          Error: {error}
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="rounded-xl border bg-white p-3 space-y-3">
          <h2 className="text-sm font-semibold">Propose Input</h2>

          <label className="text-xs text-neutral-600 block">
            Target Path
            <input
              className="mt-1 w-full border rounded px-2 py-1"
              value={targetPath}
              onChange={(e) => setTargetPath(e.target.value)}
              placeholder="rules/mamotama.conf"
            />
          </label>

          <label className="inline-flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={useLatestEvent}
              onChange={(e) => setUseLatestEvent(e.target.checked)}
            />
            Use latest `waf_block` log event
          </label>

          {!useLatestEvent && (
            <div className="grid gap-2 sm:grid-cols-2">
              <Field label="event_id" value={eventInput.event_id} onChange={(v) => updateEvent("event_id", v)} />
              <Field label="method" value={eventInput.method} onChange={(v) => updateEvent("method", v)} />
              <Field label="path" value={eventInput.path} onChange={(v) => updateEvent("path", v)} />
              <Field label="rule_id" value={eventInput.rule_id} onChange={(v) => updateEvent("rule_id", v)} />
              <Field label="status" value={eventInput.status} onChange={(v) => updateEvent("status", v)} />
              <Field label="matched_variable" value={eventInput.matched_variable} onChange={(v) => updateEvent("matched_variable", v)} />
              <label className="text-xs text-neutral-600 block sm:col-span-2">
                matched_value
                <textarea
                  className="mt-1 w-full border rounded px-2 py-1 font-mono text-xs h-20"
                  value={eventInput.matched_value}
                  onChange={(e) => updateEvent("matched_value", e.target.value)}
                />
              </label>
            </div>
          )}

          <button
            className="px-3 py-1.5 rounded-xl shadow text-sm bg-black text-white disabled:opacity-50"
            onClick={() => void onPropose()}
            disabled={proposing}
          >
            {proposing ? "Proposing..." : "Propose"}
          </button>
        </section>

        <section className="rounded-xl border bg-white p-3 space-y-3">
          <h2 className="text-sm font-semibold">Apply</h2>

          <label className="inline-flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={simulate}
              onChange={(e) => setSimulate(e.target.checked)}
            />
            Simulate only
          </label>

          <label className="text-xs text-neutral-600 block">
            approval_token
            <input
              className="mt-1 w-full border rounded px-2 py-1 font-mono text-xs"
              value={approvalToken}
              onChange={(e) => setApprovalToken(e.target.value)}
              placeholder="auto-filled from propose response"
            />
          </label>

          <button
            className="px-3 py-1.5 rounded-xl shadow text-sm bg-black text-white disabled:opacity-50"
            onClick={() => void onApply()}
            disabled={!canApply || applying}
          >
            {applying ? "Applying..." : "Apply"}
          </button>

          {applyResult && (
            <pre className="text-xs rounded border p-2 overflow-x-auto bg-neutral-50">
              {JSON.stringify(applyResult, null, 2)}
            </pre>
          )}
        </section>
      </div>

      <section className="rounded-xl border bg-white p-3 space-y-2">
        <h2 className="text-sm font-semibold">Proposal</h2>

        {!proposal && <div className="text-sm text-neutral-500">No proposal yet.</div>}

        {proposal && (
          <div className="grid gap-2">
            <Field label="id" value={proposal.id || ""} onChange={(v) => updateProposal("id", v)} />
            <Field label="title" value={proposal.title || ""} onChange={(v) => updateProposal("title", v)} />
            <Field label="summary" value={proposal.summary || ""} onChange={(v) => updateProposal("summary", v)} />
            <Field label="reason" value={proposal.reason || ""} onChange={(v) => updateProposal("reason", v)} />
            <Field
              label="confidence"
              value={String(proposal.confidence ?? "")}
              onChange={(v) => updateProposal("confidence", Number(v) || 0)}
            />
            <Field
              label="target_path"
              value={proposal.target_path || ""}
              onChange={(v) => updateProposal("target_path", v)}
            />
            <label className="text-xs text-neutral-600 block">
              rule_line
              <textarea
                className="mt-1 w-full border rounded px-2 py-1 font-mono text-xs h-28"
                value={proposal.rule_line || ""}
                onChange={(e) => updateProposal("rule_line", e.target.value)}
              />
            </label>
          </div>
        )}
      </section>

      {proposeResult && (
        <section className="rounded-xl border bg-white p-3 space-y-2">
          <h2 className="text-sm font-semibold">Last Propose Response</h2>
          <pre className="text-xs rounded border p-2 overflow-x-auto bg-neutral-50">
            {JSON.stringify(proposeResult, null, 2)}
          </pre>
        </section>
      )}
    </div>
  );
}

function Field({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <label className="text-xs text-neutral-600 block">
      {label}
      <input
        className="mt-1 w-full border rounded px-2 py-1 font-mono text-xs"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </label>
  );
}

function Badge({ color, children }: { color: "gray" | "green" | "amber"; children: ReactNode }) {
  const cls =
    color === "green"
      ? "bg-green-100 text-green-800"
      : color === "amber"
      ? "bg-amber-100 text-amber-800"
      : "bg-neutral-100 text-neutral-700";
  return <span className={`px-2 py-0.5 text-xs rounded ${cls}`}>{children}</span>;
}
