module github.com/yairfalse/elava

go 1.24.6

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/aws/aws-sdk-go-v2 v1.41.0
	github.com/aws/aws-sdk-go-v2/config v1.28.7
	github.com/aws/aws-sdk-go-v2/service/acm v1.37.15
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.33.2
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.59.1
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.58.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.61.1
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.53.2
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.218.0
	github.com/aws/aws-sdk-go-v2/service/ecs v1.69.1
	github.com/aws/aws-sdk-go-v2/service/eks v1.73.3
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.51.5
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.50.4
	github.com/aws/aws-sdk-go-v2/service/glue v1.135.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.52.2
	github.com/aws/aws-sdk-go-v2/service/kafka v1.46.6
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.42.6
	github.com/aws/aws-sdk-go-v2/service/lambda v1.70.0
	github.com/aws/aws-sdk-go-v2/service/opensearch v1.57.0
	github.com/aws/aws-sdk-go-v2/service/rds v1.88.0
	github.com/aws/aws-sdk-go-v2/service/redshift v1.61.1
	github.com/aws/aws-sdk-go-v2/service/route53 v1.61.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.68.0
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.40.2
	github.com/aws/aws-sdk-go-v2/service/sfn v1.40.2
	github.com/aws/aws-sdk-go-v2/service/sns v1.39.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.17
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.5
	github.com/prometheus/client_golang v1.23.0
	github.com/rs/zerolog v1.34.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0
	go.opentelemetry.io/otel/exporters/prometheus v0.60.0
	go.opentelemetry.io/otel/metric v1.38.0
	go.opentelemetry.io/otel/sdk v1.38.0
	go.opentelemetry.io/otel/sdk/metric v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
	golang.org/x/sync v0.19.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.3 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.48 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.22 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.14 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.7 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/otlptranslator v0.0.2 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/yairfalse/proto v0.0.2-0.20251230094149-a73dbd9008b7 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.1 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250825161204-c5933d9347a5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250825161204-c5933d9347a5 // indirect
	google.golang.org/grpc v1.75.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
