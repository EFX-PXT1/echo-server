@SETLOCAL
@REM SET ENABLE_FEATURES="delay,think,headers,env,otel,post,log"
@SET ENABLE_FEATURES="nosignals,delay,think,headers,post,otel,timeout"
@REM SET TRACE_GRPC=yes
@REM SET LOG_LEVEL=-4
@REM SET LOG_JSON=yes
@REM SET LOG_SOURCE=yes
@REM https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
@REM https://www.jaegertracing.io/docs/next-release/getting-started/#all-in-one
SET OTEL_EXPORTER_OTLP_TRACES_INSECURE=true
SET OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318/
@REM https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
SET OTEL_PROPAGATORS=tracecontext,baggage,b3multi
@echo-server.exe
@ENDLOCAL