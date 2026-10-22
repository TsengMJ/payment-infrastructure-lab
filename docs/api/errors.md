# API errors

`/v1` error responses use one JSON object. `/healthz` and `/readyz` stay plain-text probes.

`retryable` means the same request may be sent again. It does not mean a payment failed. Moving money still needs the same Idempotency-Key once that exists.

`request_id` is the `Request-Id` response header. `param` is present only when one field is at fault.

| HTTP | type | code | retryable | When |
|---|---|---|---|---|
| 400 | invalid_request_error | parameter_invalid | false | A parameter failed validation. |
| 401 | authentication_error | invalid_api_key | false | The API key is missing, malformed, or not accepted. |
| 403 | invalid_request_error | merchant_inactive | false | The key is valid and the merchant is not active. |
| 404 | invalid_request_error | resource_missing | false | The id is not in this merchant and mode. Another merchant's resource looks the same. |
| 500 | api_error | internal_error | true | The server failed. The body does not include the internal error. |
| 503 | api_error | service_unavailable | true | A dependency could not be reached. |

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "resource_missing",
    "message": "No such resource.",
    "request_id": "req_example",
    "retryable": false
  }
}
```
