package main

import (
	"context"
	"encoding/json"
	"fmt"
	_ "fmt"
	"os"
	"strconv"
	"time"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-sdk"
)

func SnapshotHandler(ctx context.Context, event MessageQueueEvent) (*Response, error) {
	// Authorization in SDK using ServiceAccount
	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		// Calling InstanceServiceAccount automatically requests IAM-token and with it constructs
		// necessary SDK credentials
		Credentials: ycsdk.InstanceServiceAccount(),
	})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	// We get the value of snaphots Time-to-live (TTL) from the environmental variable
	ttl, err := strconv.Atoi(os.Getenv("TTL"))
	if err != nil {
		return nil, err
	}

	// We calculate the exact timestamp, after which we can delete the snapshot.
	expirationTs := strconv.Itoa(int(now.Unix()) + ttl)

	// We parse json with data containing lists of folders and disks in those folders that we need to backup
	body := event.Messages[0].Details.Message.Body
	createSnapshotParams := &CreateSnapshotParams{}
	err = json.Unmarshal([]byte(body), createSnapshotParams)
	if err != nil {
		return nil, err
	}

	// .With YC SDK we create a snapshot, and label it with it's Time-to-live
	// This snapshot will not be deleted autommatically by thhe cloud itself. We will use ./delete-expired.go functions for cleaning up expired snapshots.
	snapshotOp, err := sdk.WrapOperation(sdk.Compute().Snapshot().Create(ctx, &compute.CreateSnapshotRequest{
		FolderId: createSnapshotParams.FolderId,
		DiskId:   createSnapshotParams.DiskId,
		Labels: map[string]string{
			"expiration_ts": expirationTs,
		},
	}))
	if err != nil {
		return nil, err
	}
	// If for some reason snapshot operation failed, message will come back to queue.
	// After that trigger will once again take the message from the queue, call in that functiion to retry the snapshot creation one ore time.
	if opErr := snapshotOp.Error(); opErr != nil {
		return &Response{
			StatusCode: 200,
			Body:       fmt.Sprintf("Failed to create snapshot: %s", snapshotOp.Error()),
		}, nil
	}
	meta, err := snapshotOp.Metadata()
	if err != nil {
		return nil, err
	}
	meta.(*compute.CreateSnapshotMetadata).GetSnapshotId()
	return &Response{
		StatusCode: 200,
		Body: fmt.Sprintf("Created snapshot %s from disk %s",
			meta.(*compute.CreateSnapshotMetadata).GetSnapshotId(),
			meta.(*compute.CreateSnapshotMetadata).GetDiskId()),
	}, nil
}
