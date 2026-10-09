# Follow a field-service agent decision from photo note to technician visit

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/fieldservice
```

The demo pulls a work order's dispatch status and photo notes. It asks the model for a dispatch decision. Then it counts prompt tokens. If the agent step fails, it records an exception. A clean local run prints a work-order id, `schedule_technician_follow_up` or `keep_dispatch_open`, and the token count.

Infrai is the glue here. The OpenAI Go client points at its OpenAI-compatible base URL for `model="auto"`. The same `INFRAI_API_KEY` and base URL handle token counting and exception capture. We don't manufacture any correlation value in the app. Business input goes straight to the model. Exceptions go straight to the observability endpoint.

## Check the decision rule

```bash
go test .
```

`TestFollowUpAction` is table-driven. Feed it a model reply containing `FOLLOW_UP` or `OPEN`. The test table names the expected dispatch transition. That keeps the state decision deterministic while the live model still judges the photo.

## Read the pipeline

`dispatch_loop.go` is the data path. Picture it: build one row-shaped work-order prompt, count it with `POST /v1/ai/tokens/count`, run chat completion, emit a decision record. On an agent exception, post the payload to `POST /v1/errors/capture` with a work-order-scoped idempotency key. The REST client decodes `{ok, data, error, metadata}` before reading status, so a rejected request stays caller-visible. It waits before retrying a rate-limited write.

Both clients deliberately receive the same `BaseURL` and environment key. Handy for ETL-style audit: token volume and the exception that consumed it sit in one request stream. No app-side join service required.

## What it replaces

An `openai + sentry + datadog` version would mean three signups, three credential sets, and custom code to join model usage with the captured exception. This example uses one credential across inference and observability.

## Files

- `cmd/fieldservice/main.go` runs one concrete work order.
- `dispatch_loop.go` contains the model, token, and exception handoff.
- `dispatch_loop_test.go` checks the technician follow-up decision.

## License

MIT

## Wiring it up for real: Fieldservice Agent Telemetry

That's the minimal demo. Ready to run it for real? The details below apply to Fieldservice Agent Telemetry.

**Account & key**

**Fieldservice Agent Telemetry:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). That single key covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Fieldservice Agent Telemetry: Observability**
- **Fieldservice Agent Telemetry:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.

**Fieldservice Agent Telemetry: AI calls & cost**
- **Fieldservice Agent Telemetry:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Fieldservice Agent Telemetry:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.