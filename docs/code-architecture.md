# Code architecture

## folder structure

```
cmd/
internal/
  ├── monitor/       <-- DEEP MODULE: Handles checks, scheduling & persistence logic
  │   ├── site.go
  │   ├── checker.go
  │   └── store.go
  ├── notify/        <-- DEEP MODULE: Handles dispatching alerts to channels
  │   ├── notifier.go
  │   ├── telegram.go
  │   └── webhook.go
  └── ui/            <-- Web handlers, HTMX routes & Templ components
      ├── views/
      └── handlers.go
```