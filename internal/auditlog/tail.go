package auditlog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	logging "cloud.google.com/go/logging/apiv2"
	"cloud.google.com/go/logging/apiv2/loggingpb"
	"golang.org/x/time/rate"
	"google.golang.org/genproto/googleapis/cloud/audit"
	"google.golang.org/grpc/status"
)

// Tail streams audit log entries relating to fluxcd resources. Only audit log entries relating to non-system users
// patching or creating resources are returned.
func Tail(ctx context.Context, projectID string, clusterName string, cb func(*audit.AuditLog) error) error {
	client, err := logging.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	// Limiter used to throttle log tailing restarts
	limiter := rate.NewLimiter(rate.Every(time.Second*15), 3)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err = limiter.Wait(ctx); err != nil {
			return fmt.Errorf("limit wait failed: %w", err)
		}

		if err = tailLogs(ctx, client, projectID, clusterName, cb); err != nil {
			if _, ok := status.FromError(err); ok {
				slog.Warn("gRPC request terminated, restarting", slog.Any("error", err))
				continue
			}
			return fmt.Errorf("log tailing failed: %w", err)
		}
	}
}

func tailLogs(ctx context.Context, client *logging.Client, projectID, clusterName string, cb func(*audit.AuditLog) error) error {
	stream, err := client.TailLogEntries(ctx)
	if err != nil {
		return fmt.Errorf("request to tail log entries failed: %w", err)
	}
	defer stream.CloseSend()

	req := &loggingpb.TailLogEntriesRequest{
		ResourceNames: []string{
			fmt.Sprintf("projects/%s", projectID),
		},
		Filter: strings.Join(
			[]string{
				`resource.type="k8s_cluster"`,
				fmt.Sprintf(`log_name="projects/%s/logs/cloudaudit.googleapis.com%%2Factivity"`, projectID),
				fmt.Sprintf(`resource.labels.cluster_name="%s"`, clusterName),
				`protoPayload."@type"="type.googleapis.com/google.cloud.audit.AuditLog"`,
				`protoPayload.methodName=~"io\.fluxcd\.toolkit\..*\.(patch|create)"`,
				`-protoPayload.authenticationInfo.principalEmail=~"^system:serviceaccount:flux-system:.*-controller$"`,
			},
			" AND ",
		),
	}
	if err = stream.Send(req); err != nil {
		return fmt.Errorf("stream send failed: %w", err)
	}

	return readStream(ctx, stream, cb)
}

func readStream(ctx context.Context, stream loggingpb.LoggingServiceV2_TailLogEntriesClient, cb func(*audit.AuditLog) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resp, err := stream.Recv()
			switch {
			case errors.Is(err, io.EOF):
				break
			case err != nil:
				return fmt.Errorf("stream receive failed: %w", err)
			default:
			}

			for _, entry := range resp.GetEntries() {
				payload := entry.GetProtoPayload()
				if payload == nil {
					slog.Warn("unexpected payload type")
					continue
				}

				msg, err := payload.UnmarshalNew()
				if err != nil {
					slog.Warn("failed to unmarshal payload", slog.Any("error", err))
					continue
				}

				auditLog, ok := msg.(*audit.AuditLog)
				if !ok {
					slog.Warn("unexpected payload type", slog.Any("type", fmt.Sprintf("%t", msg)))
					continue
				}

				if err = cb(auditLog); err != nil {
					return fmt.Errorf("callback failed: %w", err)
				}
			}
		}
	}
}
