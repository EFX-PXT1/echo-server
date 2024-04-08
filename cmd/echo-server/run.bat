@REM SET ENABLE_FEATURES="delay,think,headers,env,otel,post,log"
@SET ENABLE_FEATURES="delay,think,headers,post,otel"
@REM https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
@REM https://www.jaegertracing.io/docs/next-release/getting-started/#all-in-one
SET OTEL_EXPORTER_OTLP_TRACES_INSECURE=true
SET OTEL_EXPORTER_OTLP_ENDPOINT=https://localhost:14317/
@REM SET OTEL_EXPORTER_OTLP_ENDPOINT=https://localhost:4317/
@REM https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
SET OTEL_PROPAGATORS=tracecontext,baggage,b3multi
@echo-server.exe