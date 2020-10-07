package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	ycsdk "github.com/yandex-cloud/go-sdk"
)

func DeleteHandler(ctx context.Context) (*Response, error) {
	folderId := os.Getenv("FOLDER_ID")
	// Authorization in SDK using ServiceAccount
	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		// Calling InstanceServiceAccount automatically requests IAM-token and with it constructs
		// necessary SDK credentials
		Credentials: ycsdk.InstanceServiceAccount(),
	})
	if err != nil {
		return nil, err
	}

	// Create snapshot iterator wth YC SDK
	snapshotIter := sdk.Compute().Snapshot().SnapshotIterator(ctx, folderId)
	deletedIds := []string{}
	// We iterate over it
	for snapshotIter.Next() {
		snapshot := snapshotIter.Value()
		labels := snapshot.Labels
		if labels == nil {
			continue
		}
		// We check whether snapshot has label `expiration_ts`.
		expirationTsVal, ok := labels["expiration_ts"]
		if !ok {
			continue
		}
		now := time.Now()
		expirationTs, err := strconv.Atoi(expirationTsVal)
		if err != nil {
			continue
		}

		// If that label is present, and the date&time in that label are before current date, we consider that snapshot expired and remove it
		if int(now.Unix()) > expirationTs {
			op, err := sdk.WrapOperation(sdk.Compute().Snapshot().Delete(ctx, &compute.DeleteSnapshotRequest{
				SnapshotId: snapshot.Id,
			}))
			if err != nil {
				return nil, err
			}
			meta, err := op.Metadata()
			if err != nil {
				return nil, err
			}
			deletedIds = append(deletedIds, meta.(*compute.DeleteSnapshotMetadata).GetSnapshotId())
		}
	}

	return &Response{
		StatusCode: 200,
		Body:       fmt.Sprintf("Deleted expired snapshots: %s", strings.Join(deletedIds, ", ")),
	}, nil
}
