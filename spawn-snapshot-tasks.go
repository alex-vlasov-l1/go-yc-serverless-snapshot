package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	ycsdk "github.com/yandex-cloud/go-sdk"
)

var (
	endpoint = "https://message-queue.api.cloud.yandex.net"
	region   = "ru-central1"
)

func constructDiskMessage(data CreateSnapshotParams, queueUrl *string) *sqs.SendMessageInput {
	body, _ := json.Marshal(&data)
	messageBody := string(body)
	return &sqs.SendMessageInput{
		MessageBody: &messageBody,
		QueueUrl:    queueUrl,
	}
}

func SpawnHandler(ctx context.Context) (*Response, error) {
	folderId := os.Getenv("FOLDER_ID")
	mode := os.Getenv("MODE")
	queueUrl := os.Getenv("QUEUE_URL")
	onlyMarked := mode == "only-marked"

	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		// Calling InstanceServiceAccount automatically requests IAM-token and with it constructs
		// necessary SDK credentials
		Credentials: ycsdk.InstanceServiceAccount(),
	})
	if err != nil {
		return nil, err
	}

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Endpoint: &endpoint,
			Region:   &region,
		},
		SharedConfigState: session.SharedConfigEnable,
	}))

	svc := sqs.New(sess)
	// We get the iterator
	discIter := sdk.Compute().Disk().DiskIterator(ctx, folderId)
	var diskIds []string
	// And iterate over the list of all disks in Folder
	for discIter.Next() {
		d := discIter.Value()
		labels := d.GetLabels()
		ok := false
		if labels != nil {
			_, ok = labels["snapshot"]
		}
		// If variable `MODE` is set to `only-marked`,
		// then snapshots will only be created for disks with label 'snapshot' Otherwise all  disks will be backed up with snapshot

		if onlyMarked && !ok {
			continue
		}

		params := constructDiskMessage(CreateSnapshotParams{
			FolderId: folderId,
			DiskId:   d.Id,
		}, &queueUrl)
		// We send the message to Yandex Message Queuue with parameters of disk that needs to be shapshotted
		_, err = svc.SendMessage(params)
		if err != nil {
			fmt.Println("Error", err)
			return nil, err
		}
		diskIds = append(diskIds, d.Id)
	}
	return &Response{
		StatusCode: 200,
		Body:       strings.Join(diskIds, ", "),
	}, nil
}
