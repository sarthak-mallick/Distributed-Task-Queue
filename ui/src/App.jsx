import { useCallback, useEffect, useMemo, useState } from "react";

const JOB_TYPE_PRESETS = {
  weather: { city: "Austin", country_code: "US", units: "metric" },
  quote: { category: "inspirational", limit: 1 },
  exchange_rate: { base_currency: "USD", target_currency: "EUR" },
  github_user: { username: "torvalds" },
};

const TERMINAL_STATES = new Set(["completed", "failed", "not_found"]);

// createDefaultPayload returns formatted JSON payload text for one job type.
function createDefaultPayload(jobType) {
  const payload = JOB_TYPE_PRESETS[jobType] ?? {};
  return JSON.stringify(payload, null, 2);
}

// normalizeApiBaseUrl standardizes API base input for request URL assembly.
function normalizeApiBaseUrl(rawUrl) {
  const trimmed = String(rawUrl || "").trim();
  const fallback = "http://localhost:8080";
  if (!trimmed) {
    return fallback;
  }
  return trimmed.replace(/\/+$/, "");
}

// parsePayloadJSON validates payload textarea content and requires an object.
function parsePayloadJSON(payloadText) {
  let parsed;
  try {
    parsed = JSON.parse(payloadText);
  } catch (error) {
    throw new Error("payload must be valid JSON");
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
    throw new Error("payload JSON must be an object");
  }
  return parsed;
}

// isTerminalState returns true when job processing reached a finished state.
function isTerminalState(state) {
  return TERMINAL_STATES.has(String(state || "").toLowerCase());
}

// formatJson renders JSON for the details card in a readable format.
function formatJson(value) {
  if (value === null || value === undefined) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}

// App renders the Day 5 React flow for generic submit and status tracking.
export default function App() {
  const [apiBaseUrl, setApiBaseUrl] = useState("http://localhost:8080");
  const [jobType, setJobType] = useState("weather");
  const [payloadText, setPayloadText] = useState(createDefaultPayload("weather"));

  const [submitLoading, setSubmitLoading] = useState(false);
  const [submitError, setSubmitError] = useState("");
  const [submitResponse, setSubmitResponse] = useState(null);

  const [statusJobId, setStatusJobId] = useState("");
  const [statusLoading, setStatusLoading] = useState(false);
  const [statusError, setStatusError] = useState("");
  const [statusResponse, setStatusResponse] = useState(null);

  const [pollingEnabled, setPollingEnabled] = useState(true);
  const [pollIntervalMs, setPollIntervalMs] = useState(2000);

  // resolvedApiUrl stores one normalized base URL for submit/status requests.
  const resolvedApiUrl = useMemo(() => normalizeApiBaseUrl(apiBaseUrl), [apiBaseUrl]);

  // handleJobTypeChange swaps job type and resets payload to a matching template.
  const handleJobTypeChange = useCallback((nextJobType) => {
    console.info("ui submit form job_type changed", { jobType: nextJobType });
    setJobType(nextJobType);
    setPayloadText(createDefaultPayload(nextJobType));
    setSubmitError("");
  }, []);

  // handlePayloadReset restores payload JSON from the current job type preset.
  const handlePayloadReset = useCallback(() => {
    console.info("ui payload reset to preset", { jobType });
    setPayloadText(createDefaultPayload(jobType));
    setSubmitError("");
  }, [jobType]);

  // requestJobStatus calls GET /v1/jobs/{job_id}/status and updates UI state.
  const requestJobStatus = useCallback(
    async (jobIdToCheck, source) => {
      const cleanJobId = String(jobIdToCheck || "").trim();
      if (!cleanJobId) {
        setStatusError("job_id is required for status check");
        return null;
      }

      setStatusLoading(true);
      setStatusError("");
      console.info("ui status request started", { jobId: cleanJobId, source });

      try {
        const response = await fetch(`${resolvedApiUrl}/v1/jobs/${cleanJobId}/status`, {
          method: "GET",
          headers: { Accept: "application/json" },
        });

        const data = await response.json().catch(() => ({}));
        if (response.status === 404) {
          console.warn("ui status request returned not_found", { jobId: cleanJobId, source });
          setStatusResponse(data);
          return data;
        }
        if (!response.ok) {
          const detail = data?.error || `status request failed with HTTP ${response.status}`;
          throw new Error(detail);
        }

        console.info("ui status request succeeded", {
          jobId: cleanJobId,
          state: data?.state,
          progress: data?.progress_percent,
          source,
        });
        setStatusResponse(data);
        return data;
      } catch (error) {
        const message = error instanceof Error ? error.message : "unknown status request error";
        console.error("ui status request failed", { jobId: cleanJobId, source, error: message });
        setStatusError(message);
        return null;
      } finally {
        setStatusLoading(false);
      }
    },
    [resolvedApiUrl]
  );

  // handleSubmitJob sends generic POST /v1/jobs and preloads status tracking state.
  const handleSubmitJob = useCallback(
    async (event) => {
      event.preventDefault();

      setSubmitLoading(true);
      setSubmitError("");
      setSubmitResponse(null);

      let payloadObject;
      try {
        payloadObject = parsePayloadJSON(payloadText);
      } catch (error) {
        const message = error instanceof Error ? error.message : "invalid payload";
        console.warn("ui submit validation failed", { error: message });
        setSubmitError(message);
        setSubmitLoading(false);
        return;
      }

      const body = { job_type: jobType, payload: payloadObject };
      console.info("ui submit request started", { jobType, apiBaseUrl: resolvedApiUrl });

      try {
        const response = await fetch(`${resolvedApiUrl}/v1/jobs`, {
          method: "POST",
          headers: { "Content-Type": "application/json", Accept: "application/json" },
          body: JSON.stringify(body),
        });

        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
          const detail = data?.error || `submit failed with HTTP ${response.status}`;
          throw new Error(detail);
        }

        console.info("ui submit request succeeded", { jobId: data?.job_id, jobType });
        setSubmitResponse(data);
        if (data?.job_id) {
          setStatusJobId(data.job_id);
          setPollingEnabled(true);
          await requestJobStatus(data.job_id, "post-submit");
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : "unknown submit error";
        console.error("ui submit request failed", { jobType, error: message });
        setSubmitError(message);
      } finally {
        setSubmitLoading(false);
      }
    },
    [jobType, payloadText, requestJobStatus, resolvedApiUrl]
  );

  // handleCheckStatus performs one manual status lookup for the provided job ID.
  const handleCheckStatus = useCallback(() => {
    requestJobStatus(statusJobId, "manual");
  }, [requestJobStatus, statusJobId]);

  // useEffect starts/stops polling while tracking a selected job ID.
  useEffect(() => {
    if (!pollingEnabled || !statusJobId.trim()) {
      return undefined;
    }
    if (pollIntervalMs < 1000) {
      return undefined;
    }

    console.info("ui polling started", {
      jobId: statusJobId.trim(),
      intervalMs: pollIntervalMs,
    });

    const intervalId = window.setInterval(async () => {
      const latest = await requestJobStatus(statusJobId, "poll");
      if (isTerminalState(latest?.state)) {
        console.info("ui polling stopped after terminal state", {
          jobId: statusJobId.trim(),
          state: latest?.state,
        });
        setPollingEnabled(false);
      }
    }, pollIntervalMs);

    return () => {
      console.info("ui polling cleared", { jobId: statusJobId.trim() });
      window.clearInterval(intervalId);
    };
  }, [pollingEnabled, pollIntervalMs, requestJobStatus, statusJobId]);

  // progressPercent normalizes displayed progress to a bounded integer.
  const progressPercent = useMemo(() => {
    const raw = Number(statusResponse?.progress_percent || 0);
    if (Number.isNaN(raw)) {
      return 0;
    }
    return Math.max(0, Math.min(100, Math.round(raw)));
  }, [statusResponse]);

  return (
    <main className="page">
      <div className="ambient-shape ambient-shape-left" />
      <div className="ambient-shape ambient-shape-right" />

      <header className="hero">
        <p className="eyebrow">Distributed Task Queue</p>
        <h1>Job Control Console</h1>
        <p className="hero-copy">
          Submit generic jobs through the Week 1 API, then monitor state and progress in near real time.
        </p>
      </header>

      <section className="panel">
        <h2>Connection</h2>
        <label className="field">
          <span>API base URL</span>
          <input
            value={apiBaseUrl}
            onChange={(event) => setApiBaseUrl(event.target.value)}
            placeholder="http://localhost:8080"
          />
        </label>
      </section>

      <section className="panel">
        <h2>Submit Job</h2>
        <form onSubmit={handleSubmitJob} className="form-grid">
          <label className="field">
            <span>Job type</span>
            <select value={jobType} onChange={(event) => handleJobTypeChange(event.target.value)}>
              <option value="weather">weather</option>
              <option value="quote">quote</option>
              <option value="exchange_rate">exchange_rate</option>
              <option value="github_user">github_user</option>
            </select>
          </label>

          <label className="field payload-field">
            <span>Payload JSON</span>
            <textarea value={payloadText} onChange={(event) => setPayloadText(event.target.value)} rows={10} />
          </label>

          <div className="button-row">
            <button type="submit" disabled={submitLoading}>
              {submitLoading ? "Submitting..." : "Submit Job"}
            </button>
            <button type="button" className="button-ghost" onClick={handlePayloadReset} disabled={submitLoading}>
              Reset Payload
            </button>
          </div>
        </form>

        {submitError ? <p className="alert alert-error">{submitError}</p> : null}
        {submitResponse ? (
          <article className="json-card">
            <h3>Submit Response</h3>
            <pre>{formatJson(submitResponse)}</pre>
          </article>
        ) : null}
      </section>

      <section className="panel">
        <h2>Status Tracker</h2>
        <div className="status-controls">
          <label className="field">
            <span>Job ID</span>
            <input
              value={statusJobId}
              onChange={(event) => setStatusJobId(event.target.value)}
              placeholder="Paste job_id to check status"
            />
          </label>

          <label className="field">
            <span>Poll interval (ms)</span>
            <input
              type="number"
              min="1000"
              step="500"
              value={pollIntervalMs}
              onChange={(event) => setPollIntervalMs(Number(event.target.value))}
            />
          </label>
        </div>

        <div className="button-row">
          <button type="button" onClick={handleCheckStatus} disabled={statusLoading}>
            {statusLoading ? "Checking..." : "Check Status"}
          </button>
          <button type="button" className="button-ghost" onClick={() => setPollingEnabled((prev) => !prev)}>
            {pollingEnabled ? "Pause Polling" : "Resume Polling"}
          </button>
        </div>

        <p className="muted">
          Polling is <strong>{pollingEnabled ? "enabled" : "paused"}</strong>. It auto-stops on terminal states.
        </p>

        {statusError ? <p className="alert alert-error">{statusError}</p> : null}

        {statusResponse ? (
          <>
            <div className="progress-wrap" aria-label="job progress">
              <div className="progress-bar" style={{ width: `${progressPercent}%` }} />
            </div>
            <p className="muted progress-label">
              State: <strong>{statusResponse.state || "unknown"}</strong> | Progress:{" "}
              <strong>{progressPercent}%</strong>
            </p>

            <article className="json-card">
              <h3>Status Response</h3>
              <pre>{formatJson(statusResponse)}</pre>
            </article>
          </>
        ) : null}
      </section>
    </main>
  );
}
