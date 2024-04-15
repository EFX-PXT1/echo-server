#!/bin/bash
export ENABLE_FEATURES="nosignals,delay,think,headers,post,otel"
export OTEL_EXPORTER_OTLP_TRACES_INSECURE=true
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318/
# https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/#general-sdk-configuration
export OTEL_PROPAGATORS=tracecontext,baggage,b3multi
./echo-server
