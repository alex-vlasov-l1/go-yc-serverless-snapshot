This is rewritten in "Go" code from https://github.com/nikolaymatrosov/yc-serverless-snapshot
### Basic principles
By default in user cloud there is a limit on maximum amount of concurrent operations, which equals 15.
Which means, that if we want to do more than 15 disk snapshots at a time, we can't just call in function which will invoke snapshot creation for all of the disks in folder or cloud.

So keep things simple without designing "retry" conditions and waiting to snapshot to finish, we will use Yandex Message Queue


SO, the first function, which will be triggered by cron scheduler, will be pushing the messages into the Yandex Message Queue, the messages will contain tasks for the second  function.

Second function will use Yandex Message Queue messages as a trigger, to do actual useful work of creating snapshots.

In case Compute API (the one responsible for VMs, disks and snapshots) gives us an error, for example if we exceed the quota on total number of snapshots or concurrent operations, function will throw and exception.
The message will not be removed from the Queue, and after some time, will once again become available to be received. This way we provide
'retry' functionality in case something is preventing us from doing snapshots, and we want to automatically recover when conditions clear.

<img src="assets/create.png" width="474px" alt="create snapshots diagram">

Another task is to cleanup unneeded or expired snapshots. When creating shapshot, we automatically label it with  `expiration_ts` containing unix-timestamp of when this snapshot is to be deleted.
Using cron scheduler triggers, this function will delete all expired snapshots.This operation does not count towards quotas (since you are obviously decreasing the amount of snapshots)

<img src="assets/cleanup.png" width="232px" alt="cleanup snapshots diagram">

### Pre-deployment and config
#### MacOs (Linux)
We suppose that you've already installed and initialized [yc](https://cloud.yandex.com/docs/cli/quickstart) and also [s3cmd](https://cloud.yandex.com/docs/storage/tools/s3cmd). We will need both to automate our function deployment.

To deploy functions in your cloud you need to do the following:
1. Create file named `.env` based on template file `.env.template` and fill in the following:
    1. `FOLDER_ID=` with folder id where to deploy the function
    2. `AWS_ACCESS_KEY_ID=`
       `AWS_SECRET_ACCESS_KEY=`
     To get those keys create service account with role "editor" in the folder where you will be deploying your function -
     ```yc iam service-account create --name sa-snapshot \
    --description "service account for snapshot automation"
    ```
     Then create the keys:
     ```yc iam access-key create --service-account-name my-robot
     access_key:
  id: aje6t3vsbj8lp9r4vk2u
  service_account_id: ajepg0mjt06siuj65usm
  created_at: "2018-11-22T14:37:51Z"
  key_id: 0n8X6WY6S24N7OjXQ0YQ
  secret: JyTRFdqw8t1kh2-OJNz4JX5ZTz9Dj1rI9hxtzMP1
    ```
     `Key_id` would be `AWS_ACCESS_KEY_ID`, and `secret` would be `AWS_SECRET_ACCESS_KEY`

     3. `DEPLOY_BUCKET=` name of s3 Bucket where to publish Function code and binaries. Create private  bucket in the same folder and write its name.
     4. Choose either `MODE=all` or `MODE=only-marked` - in mode all snapshots will be done for every disk in folder, in only-marked modeo only for disks with label  `snapshot`
     To assign label to disk do the command: `yc compute disk add-labels --name=init-test-disk --labels=snapshot=1`
     5. `SERVICE_ACCOUNT_ID=` use id of SA that  yoy created earlier
     6. `TTL=` in Snapshot Time To Live in seconds. 1 week = 60*60*24*7 = 604800 will be TTL=604800
     7. `CRATE_CRON=` schedule for snapshot creation
        `DELETE_CRON=` schedule for finding out and removing expired snapshots.
        Both are filed in AWS-format https://cloud.yandex.com/docs/functions/concepts/trigger/timer
     8. QUEUE_URL и QUEUE_ARN will be autopopulated, leave them empty.

### Deployment
1. `./script/create.sh` Launch this script first, it will create 3 functions, message queue and 3 triggers
2. `./script/deploy.sh` Launch this script next to upload the function code to bucket, and activate them

#### Windows

You can use mingw (git bash).
To install it first do:
1. Download and insstall [GnuZip](http://gnuwin32.sourceforge.net/packages/zip.htm)
2. Add `GnuZip` to PATH.
You will need administrator account to do that.
