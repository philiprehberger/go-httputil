# Changelog

## 0.2.0

- Add `WithRetry` middleware for automatic retries on 5xx/network errors with exponential backoff
- Add `WithMetrics` middleware for collecting request method, URL, status, and duration
- Add `WithBaseURL` option to prepend a base URL to all requests
- Add `WithOnRequest` middleware for pre-request hooks
- Add `WithOnResponse` middleware for post-response hooks

## 0.1.3

- Consolidate README badges onto single line, fix CHANGELOG format

## 0.1.2

- Add Development section to README

## 0.1.0

- Initial release
- Composable RoundTripper middleware chain
- Built-in middlewares: bearer token, request ID, logging, timeout, headers
