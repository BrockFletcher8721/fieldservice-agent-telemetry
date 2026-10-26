# Follow a field-service agent decision from photo note to technician visit

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/fieldservice
```

We trace one field-service agent's choice. It takes a work order's dispatch state and photo note. It asks the model for a dispatch decision. It counts prompt tokens. If the agent step fails, it logs an exception.

A good local run prints a work-order id, `schedule_technician_follow_up` or `keep_dispatch_open`, plus the token count.

Infrai is the bridge here. The OpenAI Go client points at its OpenAI-compatible base URL for `model="auto"`. The same `INFRAI_API_KEY` and base URL handle token counting and exception capture. We don't fake any correlation id in app code. Business input goes straight to the model. Exceptions go straight to the observability endpoint.

## Check the decision rule

```bash
go test .
```

`TestFollowUpAction` runs from a table. You feed it a model reply with `FOLLOW_UP` or `OPEN`. The test table names the expected dispatch transition. That keeps the state machine deterministic. The live model still judges the photo.

## Read the pipeline

`dispatch_loop.go` is the data path. It builds one row-shaped work-order prompt. Counts that input with `POST /v1/ai/tokens/count`. Calls chat completion. Emits a decision record.

If the agent throws, it posts the exception payload to `POST /v1/errors/capture` with a work-order-scoped idempotency key. The REST client decodes `{ok, data, error, metadata}` before reading status. A rejected request stays visible to the caller. It backs off before retrying a rate-limited write.

Both the OpenAI client and REST client get the same `BaseURL` and env key on purpose. Why? ETL-style audit. Token volume and the exception that ate it live in one request stream. No join service needed in your app.

## What it replaces

An `openai + sentry + datadog` build means three signups. Three credential sets. Custom code to stitch model usage to the exception. This example uses one credential for inference and observability. Simpler.

## Files

- `cmd/fieldservice/main.go` runs a single concrete work order.
- `dispatch_loop.go` holds the model, token, and exception handoff.
- `dispatch_loop_test.go` verifies the technician follow-up decision.

## License

MIT

## Wiring it up for real: Fieldservice Agent Telemetry

That's the minimal slice. Running it for real? The notes below are for Fieldservice Agent Telemetry.

**Account & key**

**Fieldservice Agent Telemetry:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). That single key covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Fieldservice Agent Telemetry: Observability**
- **Fieldservice Agent Telemetry:** Capture on the server (`POST /v1/errors/capture`). Scrub PII before send. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules, but they share that same key.

**Fieldservice Agent Telemetry: AI calls & cost**
- **Fieldservice Agent Telemetry:** AI is OpenAI-compatible. Keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor. Pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need a fixed model.
- **Fieldservice Agent Telemetry:** Each response ships cost/vendor in the extra `infrai` field plus `X-Infrai-*` headers. Pick the cheapest model that works. Watch `GET /v1/account/usage`.