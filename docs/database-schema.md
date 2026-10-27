- websites
  - id int
  - name string
  - base_url string

- endpoints
  - id int
  - path string
  - method string default GET (only get for now)
  - website_id foreignkey
  - interval_check_min int (keep it as minutes, 1 minutes minimum)
  - timeout_sec int
  - enabled bool

- expectations
  - id int
  - criteria string
  - expected_value string
  - endpoint_id foreign key

- incidents
  - id int
  - reason string
  - resolved_at nullable
  - started_at
  - endpoint_id foreignkey

- check_results
  - id int
  - endpoint_id foreign key
  - duration_ms int
  - code int | nullable (200, 201, ...)
  - error string | nullable (http error or assertion fail)
  - error_body string | nullable (stringified body)
  - checked_at

