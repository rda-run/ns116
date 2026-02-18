package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"

	"ns116/internal/config"
	"ns116/internal/database"
	"ns116/internal/model"
)

type DNSService struct {
	client       *route53.Client
	allowedZones map[string]string
	db           *database.DB
}

func NewDNSService(cfg *config.Config, db *database.DB) (*DNSService, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(cfg.AWS.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AWS.AccessKeyID,
				cfg.AWS.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	allowed := make(map[string]string)
	for _, z := range cfg.HostedZones {
		allowed[z.ID] = z.Label
	}

	return &DNSService{
		client:       route53.NewFromConfig(awsCfg),
		allowedZones: allowed,
		db:           db,
	}, nil
}

func (s *DNSService) ListZones(ctx context.Context) ([]model.HostedZone, error) {
	if zones, ok := s.db.GetCachedZones(); ok {
		return zones, nil
	}

	result, err := s.client.ListHostedZones(ctx, &route53.ListHostedZonesInput{})
	if err != nil {
		return nil, err
	}

	var zones []model.HostedZone
	for _, z := range result.HostedZones {
		zoneID := extractZoneID(*z.Id)

		if len(s.allowedZones) > 0 {
			if _, ok := s.allowedZones[zoneID]; !ok {
				continue
			}
		}

		zones = append(zones, model.HostedZone{
			ID:          zoneID,
			Name:        *z.Name,
			RecordCount: *z.ResourceRecordSetCount,
			Comment:     safeComment(z.Config),
			Label:       s.allowedZones[zoneID],
		})
	}

	_ = s.db.CacheZones(zones)
	return zones, nil
}

func (s *DNSService) GetZone(ctx context.Context, zoneID string) (model.HostedZone, error) {
	if !s.isAllowed(zoneID) {
		return model.HostedZone{}, fmt.Errorf("zone %s is not in the allowed list", zoneID)
	}

	result, err := s.client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: aws.String(zoneID),
	})
	if err != nil {
		return model.HostedZone{}, err
	}

	return model.HostedZone{
		ID:          zoneID,
		Name:        *result.HostedZone.Name,
		RecordCount: *result.HostedZone.ResourceRecordSetCount,
		Comment:     safeComment(result.HostedZone.Config),
		Label:       s.allowedZones[zoneID],
	}, nil
}

func (s *DNSService) ListRecords(ctx context.Context, zoneID string) ([]model.DNSRecord, error) {
	if !s.isAllowed(zoneID) {
		return nil, fmt.Errorf("zone %s is not in the allowed list", zoneID)
	}

	if records, ok := s.db.GetCachedRecords(zoneID); ok {
		return records, nil
	}

	var records []model.DNSRecord
	var nextName *string
	var nextType types.RRType

	for {
		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId: aws.String(zoneID),
		}
		if nextName != nil {
			input.StartRecordName = nextName
			input.StartRecordType = nextType
		}

		result, err := s.client.ListResourceRecordSets(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, rrs := range result.ResourceRecordSets {
			rec := model.DNSRecord{
				Name: *rrs.Name,
				Type: string(rrs.Type),
			}

			if rrs.AliasTarget != nil {
				rec.IsAlias = true
				rec.AliasTarget = *rrs.AliasTarget.DNSName
				rec.AliasZoneID = *rrs.AliasTarget.HostedZoneId
			} else {
				if rrs.TTL != nil {
					rec.TTL = *rrs.TTL
				}
				for _, r := range rrs.ResourceRecords {
					rec.Values = append(rec.Values, *r.Value)
				}
			}

			records = append(records, rec)
		}

		if !result.IsTruncated {
			break
		}
		nextName = result.NextRecordName
		nextType = result.NextRecordType
	}

	_ = s.db.CacheRecords(zoneID, records)
	return records, nil
}

func (s *DNSService) ChangeRecord(ctx context.Context, zoneID string, req model.RecordChangeRequest) error {
	if !s.isAllowed(zoneID) {
		return fmt.Errorf("zone %s is not in the allowed list", zoneID)
	}

	var action types.ChangeAction
	switch req.Action {
	case "CREATE":
		action = types.ChangeActionCreate
	case "UPSERT":
		action = types.ChangeActionUpsert
	case "DELETE":
		action = types.ChangeActionDelete
	default:
		return fmt.Errorf("invalid action: %s", req.Action)
	}

	var resourceRecords []types.ResourceRecord
	for _, v := range req.Values {
		resourceRecords = append(resourceRecords, types.ResourceRecord{
			Value: aws.String(v),
		})
	}

	_, err := s.client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Comment: aws.String("Changed via NS116"),
			Changes: []types.Change{
				{
					Action: action,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name:            aws.String(req.Name),
						Type:            types.RRType(req.Type),
						TTL:             aws.Int64(req.TTL),
						ResourceRecords: resourceRecords,
					},
				},
			},
		},
	})

	s.db.InvalidateRecordCache(zoneID)
	return err
}

func (s *DNSService) isAllowed(zoneID string) bool {
	if len(s.allowedZones) == 0 {
		return true
	}
	_, ok := s.allowedZones[zoneID]
	return ok
}

func extractZoneID(fullID string) string {
	parts := strings.Split(fullID, "/")
	return parts[len(parts)-1]
}

func safeComment(cfg *types.HostedZoneConfig) string {
	if cfg != nil && cfg.Comment != nil {
		return *cfg.Comment
	}
	return ""
}
