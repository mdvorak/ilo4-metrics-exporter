module ilo4-metrics-exporter

go 1.15

require (
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.3.0
	github.com/namsral/flag v1.7.4-pre
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/client_model v0.2.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20201022231255-08b38378de70
	google.golang.org/protobuf v1.25.0
	gopkg.in/fsnotify.v1 v1.4.9
)

replace gopkg.in/fsnotify.v1 => gopkg.in/fsnotify.v1 v1.4.7
