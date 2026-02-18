import {
  ApolloClient,
  HttpLink,
  InMemoryCache,
  gql,
  split,
} from "@apollo/client";
import { GraphQLWsLink } from "@apollo/client/link/subscriptions";
import { ApolloProvider, useLazyQuery, useMutation, useSubscription } from "@apollo/client/react";
import { getMainDefinition } from "@apollo/client/utilities";
import { createClient } from "graphql-ws";
import { useCallback, useEffect, useMemo, useState } from "react";

const JOB_TYPE_PRESETS = {
  weather: { city: "Austin", country_code: "US", units: "metric" },
  quote: { category: "inspirational", limit: 1 },
  exchange_rate: { base_currency: "USD", target_currency: "EUR" },
  github_user: { username: "torvalds" },
};

const TERMINAL_STATES = new Set(["completed", "failed", "not_found"]);

const SUBMIT_JOB_MUTATION = gql`
  mutation SubmitJob($input: SubmitJobInput!) {
    submitJob(input: $input) {
      jobId
      traceId
      jobType
      state
      submittedAt
      message
    }
  }
`;

const JOB_STATUS_QUERY = gql`
  query JobStatus($jobId: ID!) {
    jobStatus(jobId: $jobId) {
      jobId
      state
      progressPercent
      message
      timestamp
    }
  }
`;

const JOB_PROGRESS_SUBSCRIPTION = gql`
  subscription JobProgress($jobId: ID!) {
    jobProgress(jobId: $jobId) {
      jobId
      state
      progressPercent
      message
      timestamp
    }
  }
`;

// createDefaultPayload returns formatted JSON payload text for one job type.
function createDefaultPayload(jobType) {
  const payload = JOB_TYPE_PRESETS[jobType] ?? {};
  return JSON.stringify(payload, null, 2);
}

// normalizeApiBaseUrl standardizes API base input for GraphQL endpoint assembly.
function normalizeApiBaseUrl(rawUrl) {
  const trimmed = String(rawUrl || "").trim();
  const fallback = "http://localhost:8080";
  if (!trimmed) {
    return fallback;
  }
  return trimmed.replace(/\/+$/, "");
}

// toWebSocketUrl converts API base URL into GraphQL websocket endpoint URL.
function toWebSocketUrl(apiBaseUrl) {
  const url = new URL(apiBaseUrl);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = "/graphql/ws";
  url.search = "";
  url.hash = "";
  return url.toString();
}

// createApolloBundle creates Apollo client and websocket disposable resources.
function createApolloBundle(apiBaseUrl) {
  const httpLink = new HttpLink({ uri: `${apiBaseUrl}/graphql` });

  const wsClient = createClient({
    url: toWebSocketUrl(apiBaseUrl),
    lazy: true,
    retryAttempts: 3,
  });
  const wsLink = new GraphQLWsLink(wsClient);

  const link = split(
    ({ query }) => {
      const definition = getMainDefinition(query);
      return definition.kind === "OperationDefinition" && definition.operation === "subscription";
    },
    wsLink,
    httpLink
  );

  const client = new ApolloClient({
    link,
    cache: new InMemoryCache(),
    defaultOptions: {
      query: { fetchPolicy: "no-cache" },
      watchQuery: { fetchPolicy: "no-cache" },
    },
  });

  return {
    client,
    dispose: () => {
      wsClient.dispose();
      client.stop();
    },
  };
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

// App wires Apollo GraphQL client setup and renders the Week 2 console.
export default function App() {
  const [apiBaseUrl, setApiBaseUrl] = useState("http://localhost:8080");
  const resolvedApiUrl = useMemo(() => normalizeApiBaseUrl(apiBaseUrl), [apiBaseUrl]);

  const apolloBundle = useMemo(() => createApolloBundle(resolvedApiUrl), [resolvedApiUrl]);

  useEffect(
    () => () => {
      apolloBundle.dispose();
    },
    [apolloBundle]
  );

  return (
    <ApolloProvider client={apolloBundle.client}>
      <GraphQLConsole
        apiBaseUrl={apiBaseUrl}
        onApiBaseUrlChange={setApiBaseUrl}
        resolvedApiUrl={resolvedApiUrl}
      />
    </ApolloProvider>
  );
}

// GraphQLConsole renders submit/status UX backed by Apollo operations and subscriptions.
function GraphQLConsole({ apiBaseUrl, onApiBaseUrlChange, resolvedApiUrl }) {
  const [jobType, setJobType] = useState("weather");
  const [payloadText, setPayloadText] = useState(createDefaultPayload("weather"));

  const [submitError, setSubmitError] = useState("");
  const [submitResponse, setSubmitResponse] = useState(null);

  const [statusJobId, setStatusJobId] = useState("");
  const [statusError, setStatusError] = useState("");
  const [statusResponse, setStatusResponse] = useState(null);

  const [submitJob, { loading: submitLoading }] = useMutation(SUBMIT_JOB_MUTATION);
  const [fetchStatus, { loading: statusLoading }] = useLazyQuery(JOB_STATUS_QUERY);

  const liveJobId = statusJobId.trim();
  const {
    data: subscriptionData,
    error: subscriptionError,
    loading: subscriptionLoading,
  } = useSubscription(JOB_PROGRESS_SUBSCRIPTION, {
    variables: { jobId: liveJobId },
    skip: liveJobId.length === 0,
  });

  useEffect(() => {
    if (subscriptionData?.jobProgress) {
      setStatusError("");
      setStatusResponse(subscriptionData.jobProgress);
    }
  }, [subscriptionData]);

  useEffect(() => {
    if (subscriptionError) {
      setStatusError(subscriptionError.message);
    }
  }, [subscriptionError]);

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

  // requestJobStatus performs one GraphQL jobStatus query.
  const requestJobStatus = useCallback(
    async (jobIdToCheck, source) => {
      const cleanJobId = String(jobIdToCheck || "").trim();
      if (!cleanJobId) {
        setStatusError("jobId is required for status check");
        return null;
      }

      setStatusError("");
      console.info("ui graphql jobStatus query started", { jobId: cleanJobId, source });

      try {
        const result = await fetchStatus({ variables: { jobId: cleanJobId } });
        const status = result.data?.jobStatus;
        if (!status) {
          throw new Error("jobStatus response was empty");
        }

        setStatusResponse(status);
        console.info("ui graphql jobStatus query succeeded", {
          jobId: cleanJobId,
          state: status.state,
          progress: status.progressPercent,
          source,
        });
        return status;
      } catch (error) {
        const message = error instanceof Error ? error.message : "unknown status request error";
        console.error("ui graphql jobStatus query failed", { jobId: cleanJobId, source, error: message });
        setStatusError(message);
        return null;
      }
    },
    [fetchStatus]
  );

  // handleSubmitJob sends GraphQL submitJob mutation and starts live subscription tracking.
  const handleSubmitJob = useCallback(
    async (event) => {
      event.preventDefault();
      setSubmitError("");
      setSubmitResponse(null);

      let payloadObject;
      try {
        payloadObject = parsePayloadJSON(payloadText);
      } catch (error) {
        const message = error instanceof Error ? error.message : "invalid payload";
        setSubmitError(message);
        return;
      }

      console.info("ui graphql submitJob mutation started", { jobType, apiBaseUrl: resolvedApiUrl });

      try {
        const result = await submitJob({
          variables: {
            input: {
              jobType,
              payload: payloadObject,
            },
          },
        });

        const submit = result.data?.submitJob;
        if (!submit) {
          throw new Error("submitJob response was empty");
        }

        setSubmitResponse(submit);
        if (submit.jobId) {
          setStatusJobId(submit.jobId);
          await requestJobStatus(submit.jobId, "post-submit");
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : "unknown submit error";
        console.error("ui graphql submitJob mutation failed", { jobType, error: message });
        setSubmitError(message);
      }
    },
    [jobType, payloadText, requestJobStatus, resolvedApiUrl, submitJob]
  );

  // handleCheckStatus performs one manual jobStatus query for the selected job ID.
  const handleCheckStatus = useCallback(() => {
    requestJobStatus(statusJobId, "manual");
  }, [requestJobStatus, statusJobId]);

  // progressPercent normalizes displayed progress to a bounded integer.
  const progressPercent = useMemo(() => {
    const raw = Number(statusResponse?.progressPercent || 0);
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
        <h1>GraphQL Operations Console</h1>
        <p className="hero-copy">
          Submit jobs via GraphQL mutation, query current status, and stream live progress through WebSocket
          subscriptions.
        </p>
      </header>

      <section className="panel">
        <h2>Connection</h2>
        <label className="field">
          <span>API base URL</span>
          <input
            value={apiBaseUrl}
            onChange={(event) => onApiBaseUrlChange(event.target.value)}
            placeholder="http://localhost:8080"
          />
        </label>
        <p className="muted">GraphQL HTTP: {resolvedApiUrl}/graphql | WebSocket: {toWebSocketUrl(resolvedApiUrl)}</p>
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
              placeholder="Paste jobId to query and subscribe"
            />
          </label>
        </div>

        <div className="button-row">
          <button type="button" onClick={handleCheckStatus} disabled={statusLoading}>
            {statusLoading ? "Checking..." : "Check Status"}
          </button>
        </div>

        <p className="muted">
          Live subscription: <strong>{liveJobId ? (subscriptionLoading ? "connecting" : "active") : "idle"}</strong>.
          {" "}
          Updates stop automatically on terminal states.
        </p>

        {statusError ? <p className="alert alert-error">{statusError}</p> : null}

        {statusResponse ? (
          <>
            <div className="progress-wrap" aria-label="job progress">
              <div className="progress-bar" style={{ width: `${progressPercent}%` }} />
            </div>
            <p className="muted progress-label">
              State: <strong>{statusResponse.state || "unknown"}</strong> | Progress: <strong>{progressPercent}%</strong>
              {isTerminalState(statusResponse.state) ? " | Terminal" : ""}
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
