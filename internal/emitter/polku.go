// Package emitter provides output backends for Elava scan results.
package emitter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	elavapb "github.com/yairfalse/proto/gen/go/elava/v1"
	polkupb "github.com/yairfalse/proto/gen/go/polku/v1"

	"github.com/yairfalse/elava/pkg/resource"
)

// PolkuEmitter streams events to POLKU for AHTI integration.
type PolkuEmitter struct {
	client polkupb.GatewayClient
	stream polkupb.Gateway_StreamEventsClient
	conn   *grpc.ClientConn
	config PolkuConfig

	// Batching
	buffer []*elavapb.RawCloudEvent
	mu     sync.Mutex

	// Background flush
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PolkuConfig configures the POLKU emitter.
type PolkuConfig struct {
	Addr          string        // POLKU gRPC address (e.g., "polku:50051")
	TLSCert       string        // Client certificate path (optional)
	TLSKey        string        // Client key path (optional)
	TLSCA         string        // CA certificate path (optional)
	BatchSize     int           // Events per batch (default: 100)
	FlushInterval time.Duration // Flush interval (default: 1s)
	Insecure      bool          // Use insecure connection (for development)
}

// DefaultPolkuConfig returns sensible defaults.
func DefaultPolkuConfig() PolkuConfig {
	return PolkuConfig{
		Addr:          "localhost:50051",
		BatchSize:     100,
		FlushInterval: time.Second,
		Insecure:      false,
	}
}

// NewPolkuEmitter creates a new POLKU emitter.
func NewPolkuEmitter(cfg PolkuConfig) (*PolkuEmitter, error) {
	// Apply defaults
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Second
	}

	// Set up transport credentials
	var creds credentials.TransportCredentials
	if cfg.Insecure {
		creds = insecure.NewCredentials()
	} else if cfg.TLSCA != "" {
		var err error
		creds, err = credentials.NewClientTLSFromFile(cfg.TLSCA, "")
		if err != nil {
			return nil, fmt.Errorf("load TLS CA: %w", err)
		}
	} else {
		// Use system CA pool
		creds = credentials.NewTLS(nil)
	}

	// Connect to POLKU
	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("dial polku: %w", err)
	}

	client := polkupb.NewGatewayClient(conn)

	// Open bidirectional stream
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.StreamEvents(ctx)
	if err != nil {
		cancel()
		conn.Close()
		return nil, fmt.Errorf("open stream: %w", err)
	}

	e := &PolkuEmitter{
		client: client,
		stream: stream,
		conn:   conn,
		config: cfg,
		buffer: make([]*elavapb.RawCloudEvent, 0, cfg.BatchSize),
		ctx:    ctx,
		cancel: cancel,
	}

	// Start background flush loop
	e.wg.Add(1)
	go e.flushLoop()

	// Start ack receiver
	e.wg.Add(1)
	go e.receiveAcks()

	log.Info().
		Str("addr", cfg.Addr).
		Int("batch_size", cfg.BatchSize).
		Dur("flush_interval", cfg.FlushInterval).
		Msg("connected to POLKU")

	return e, nil
}

// Emit converts resources to proto events and buffers them for streaming.
func (e *PolkuEmitter) Emit(ctx context.Context, result resource.ScanResult) error {
	if result.Error != nil {
		// Don't emit failed scans
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, r := range result.Resources {
		event := e.toRawEvent(r)
		e.buffer = append(e.buffer, event)

		if len(e.buffer) >= e.config.BatchSize {
			if err := e.flushLocked(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

// toRawEvent converts a Resource to a RawCloudEvent proto.
func (e *PolkuEmitter) toRawEvent(r resource.Resource) *elavapb.RawCloudEvent {
	event := &elavapb.RawCloudEvent{
		Id:           r.ID,
		Timestamp:    timestamppb.New(r.ScannedAt),
		Provider:     toCloudProvider(r.Provider),
		ResourceType: r.Type,
		Region:       r.Region,
		AccountId:    r.Account,
		ResourceId:   r.ID,
		ResourceName: r.Name,
		Tags:         r.Labels,
		ChangeType:   elavapb.ChangeType_CHANGE_TYPE_SNAPSHOT, // Scans are snapshots
	}

	// Set provider-specific data based on resource type and provider
	e.setProviderData(event, r)

	return event
}

// setProviderData populates the provider-specific oneof based on resource attributes.
func (e *PolkuEmitter) setProviderData(event *elavapb.RawCloudEvent, r resource.Resource) {
	switch r.Provider {
	case "aws":
		e.setAwsData(event, r)
	case "gcp":
		e.setGcpData(event, r)
	case "azure":
		e.setAzureData(event, r)
	}
}

func (e *PolkuEmitter) setAwsData(event *elavapb.RawCloudEvent, r resource.Resource) {
	aws := &elavapb.AwsData{
		Arn:              r.Attrs["arn"],
		AvailabilityZone: r.Attrs["availability_zone"],
		VpcId:            r.Attrs["vpc_id"],
		SubnetId:         r.Attrs["subnet_id"],
	}

	switch r.Type {
	case "ec2":
		aws.Resource = &elavapb.AwsData_Ec2Instance{
			Ec2Instance: &elavapb.Ec2InstanceData{
				InstanceId:   r.ID,
				InstanceType: r.Attrs["instance_type"],
				State:        r.Status,
				PrivateIp:    r.Attrs["private_ip"],
				PublicIp:     r.Attrs["public_ip"],
				AmiId:        r.Attrs["ami_id"],
				IamRole:      r.Attrs["iam_role"],
			},
		}
	case "rds":
		aws.Resource = &elavapb.AwsData_RdsInstance{
			RdsInstance: &elavapb.RdsInstanceData{
				DbInstanceId:  r.ID,
				Engine:        r.Attrs["engine"],
				EngineVersion: r.Attrs["engine_version"],
				InstanceClass: r.Attrs["instance_class"],
				Status:        r.Status,
				Endpoint:      r.Attrs["endpoint"],
			},
		}
	case "s3":
		aws.Resource = &elavapb.AwsData_S3Bucket{
			S3Bucket: &elavapb.S3BucketData{
				BucketName: r.Name,
				Region:     r.Region,
			},
		}
	case "lambda":
		aws.Resource = &elavapb.AwsData_LambdaFunction{
			LambdaFunction: &elavapb.LambdaFunctionData{
				FunctionName: r.Name,
				Runtime:      r.Attrs["runtime"],
				Handler:      r.Attrs["handler"],
			},
		}
	case "ecs_service":
		aws.Resource = &elavapb.AwsData_EcsService{
			EcsService: &elavapb.EcsServiceData{
				ClusterName: r.Attrs["cluster_name"],
				ServiceName: r.Name,
				LaunchType:  r.Attrs["launch_type"],
			},
		}
	case "eks":
		aws.Resource = &elavapb.AwsData_EksCluster{
			EksCluster: &elavapb.EksClusterData{
				ClusterName: r.Name,
				Version:     r.Attrs["version"],
				Status:      r.Status,
				Endpoint:    r.Attrs["endpoint"],
			},
		}
	case "elb":
		aws.Resource = &elavapb.AwsData_Elb{
			Elb: &elavapb.ElbData{
				LoadBalancerName: r.Name,
				Type:             r.Attrs["type"],
				Scheme:           r.Attrs["scheme"],
				State:            r.Status,
				DnsName:          r.Attrs["dns_name"],
			},
		}
	}

	event.Data = &elavapb.RawCloudEvent_Aws{Aws: aws}
}

func (e *PolkuEmitter) setGcpData(event *elavapb.RawCloudEvent, r resource.Resource) {
	gcp := &elavapb.GcpData{
		ProjectId: r.Account,
		Zone:      r.Attrs["zone"],
		SelfLink:  r.Attrs["self_link"],
	}

	switch r.Type {
	case "gce":
		gcp.Resource = &elavapb.GcpData_GceInstance{
			GceInstance: &elavapb.GceInstanceData{
				InstanceId:  r.ID,
				MachineType: r.Attrs["machine_type"],
				Status:      r.Status,
				InternalIp:  r.Attrs["internal_ip"],
				ExternalIp:  r.Attrs["external_ip"],
			},
		}
	case "gke":
		gcp.Resource = &elavapb.GcpData_GkeCluster{
			GkeCluster: &elavapb.GkeClusterData{
				ClusterName:   r.Name,
				MasterVersion: r.Attrs["version"],
				Status:        r.Status,
				Endpoint:      r.Attrs["endpoint"],
			},
		}
	case "cloudsql":
		gcp.Resource = &elavapb.GcpData_CloudSql{
			CloudSql: &elavapb.CloudSqlData{
				InstanceName:    r.Name,
				DatabaseVersion: r.Attrs["database_version"],
				State:           r.Status,
			},
		}
	case "gcs":
		gcp.Resource = &elavapb.GcpData_Gcs{
			Gcs: &elavapb.GcsData{
				BucketName:   r.Name,
				Location:     r.Region,
				StorageClass: r.Attrs["storage_class"],
			},
		}
	}

	event.Data = &elavapb.RawCloudEvent_Gcp{Gcp: gcp}
}

func (e *PolkuEmitter) setAzureData(event *elavapb.RawCloudEvent, r resource.Resource) {
	azure := &elavapb.AzureData{
		SubscriptionId: r.Account,
		ResourceGroup:  r.Attrs["resource_group"],
		Location:       r.Region,
		ResourceId:     r.ID,
	}

	switch r.Type {
	case "vm":
		azure.Resource = &elavapb.AzureData_Vm{
			Vm: &elavapb.AzureVmData{
				VmName:    r.Name,
				VmSize:    r.Attrs["vm_size"],
				PrivateIp: r.Attrs["private_ip"],
				PublicIp:  r.Attrs["public_ip"],
			},
		}
	case "aks":
		azure.Resource = &elavapb.AzureData_AksCluster{
			AksCluster: &elavapb.AksClusterData{
				ClusterName:       r.Name,
				KubernetesVersion: r.Attrs["version"],
			},
		}
	}

	event.Data = &elavapb.RawCloudEvent_Azure{Azure: azure}
}

func toCloudProvider(provider string) elavapb.CloudProvider {
	switch provider {
	case "aws":
		return elavapb.CloudProvider_CLOUD_PROVIDER_AWS
	case "gcp":
		return elavapb.CloudProvider_CLOUD_PROVIDER_GCP
	case "azure":
		return elavapb.CloudProvider_CLOUD_PROVIDER_AZURE
	default:
		return elavapb.CloudProvider_CLOUD_PROVIDER_UNSPECIFIED
	}
}

// flushLoop periodically flushes buffered events.
func (e *PolkuEmitter) flushLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			// Final flush on shutdown
			e.mu.Lock()
			_ = e.flushLocked(context.Background())
			e.mu.Unlock()
			return
		case <-ticker.C:
			e.mu.Lock()
			if err := e.flushLocked(context.Background()); err != nil {
				log.Error().Err(err).Msg("flush to POLKU failed")
			}
			e.mu.Unlock()
		}
	}
}

// flushLocked sends buffered events to POLKU. Must hold e.mu.
func (e *PolkuEmitter) flushLocked(ctx context.Context) error {
	if len(e.buffer) == 0 {
		return nil
	}

	// Serialize events to bytes
	batch := &polkupb.IngestBatch{
		Source:  "elava",
		Cluster: "", // TODO: get from config
		Payload: &polkupb.IngestBatch_Raw{
			Raw: &polkupb.RawPayload{
				Data:   serializeEventBatch(e.buffer),
				Format: "protobuf",
			},
		},
	}

	if err := e.stream.Send(batch); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	log.Debug().
		Int("events", len(e.buffer)).
		Msg("flushed events to POLKU")

	// Clear buffer
	e.buffer = e.buffer[:0]
	return nil
}

// serializeEventBatch serializes events to protobuf bytes.
func serializeEventBatch(events []*elavapb.RawCloudEvent) []byte {
	batch := &elavapb.EventBatch{
		Events: events,
		Source: "elava",
	}
	data, _ := proto.Marshal(batch)
	return data
}

// receiveAcks handles acknowledgments from POLKU.
func (e *PolkuEmitter) receiveAcks() {
	defer e.wg.Done()

	for {
		ack, err := e.stream.Recv()
		if err != nil {
			if e.ctx.Err() != nil {
				// Context cancelled, normal shutdown
				return
			}
			log.Error().Err(err).Msg("receive ack from POLKU failed")
			return
		}

		if len(ack.Errors) > 0 {
			for _, e := range ack.Errors {
				log.Warn().
					Str("event_id", e.EventId).
					Str("code", e.Code).
					Str("message", e.Message).
					Msg("POLKU rejected event")
			}
		}
	}
}

// Close shuts down the emitter gracefully.
func (e *PolkuEmitter) Close() error {
	e.cancel()
	e.wg.Wait()

	if err := e.stream.CloseSend(); err != nil {
		log.Warn().Err(err).Msg("close stream")
	}

	if err := e.conn.Close(); err != nil {
		return fmt.Errorf("close connection: %w", err)
	}

	log.Info().Msg("disconnected from POLKU")
	return nil
}
