# Payment provider change procedure

1. Add provider behavior behind the `PaymentProvider` interface.
2. Validate amount and currency before contacting the provider.
3. Preserve the idempotency key across retries and return the stored result for
   a repeated key.
4. Add or update tests for invalid requests, provider errors, and duplicate
   requests.
