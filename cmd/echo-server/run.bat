@REM SET ENABLE_FEATURES="delay,think,headers,env,otel,post,log"
@SET ENABLE_FEATURES="nosignals,delay,think,headers,post,otel"
@REM https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
@REM https://www.jaegertracing.io/docs/next-release/getting-started/#all-in-one
SET OTEL_EXPORTER_OTLP_TRACES_INSECURE=true
SET OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318/
@REM https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
SET OTEL_PROPAGATORS=tracecontext,baggage,b3multi
@echo-server.exe
