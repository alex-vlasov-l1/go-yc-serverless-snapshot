This is English translation of https://github.com/nikolaymatrosov/yc-serverless-snapshot

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

We suppose that you've already installed and initialized [yc](https://cloud.yandex.com/docs/cli/quickstart) and also [s3cmd](https://cloud.yandex.com/docs/storage/tools/s3cmd). 

We will need both to automate our function deployment.

To deploy functions in your cloud you need to do the following:

1. Create file named `.env` based on template file `.env.template` and fill in the following:

    1.1 Fill in `FOLDER_ID=` with folder id where to deploy the function
   
    1.2 Fill in`AWS_ACCESS_KEY_ID=` and `AWS_SECRET_ACCESS_KEY=` 
        To get those keys create service account with role "editor" in the folder where you will be deploying your function
        
<details><summary>YC comand and output</summary>

``` 
yc iam service-account create --name sa-snapshot 
--description "service account for snapshot automation" --format=json
```
           
```json
{ 
"id": "aje82e80o9roadhod6bl",
"folder_id": "b1gul7j7tkf53j6imep0",
"created_at": "2020-10-07T01:18:02Z",
"name": "sa-snapshot",
"description": "service account for snapshot automation" 
}
```
</details>        


<details><summary>Then create the keys:</summary>

```
yc iam access-key create --service-account-name sa-snapshot --format=json
```
          
```json
{ 
"access_key": 
             { 
               "id": "ajeup40ardboshpnifcn",
               "service_account_id": "aje82e80o9roadhod6bl",
               "created_at": "2020-10-07T01:20:11Z",
               "key_id": "qPKMRDUcwxPWHQ1D7av4 
             },
 "secret": "somesecrett" 
} 
```
</details> 

Where `Key_id` would be `AWS_ACCESS_KEY_ID`, and `secret` would be `AWS_SECRET_ACCESS_KEY`
       
   1.3 Fill in `DEPLOY_BUCKET=` name of s3 Bucket where to publish Function code and binaries. Create private  bucket in the same folder and write its name.
    
   1.4 Choose either `MODE=all` or `MODE=only-marked` - in mode all snapshots will be done for every disk in folder, in only-marked modeo only for disks with              label `snapshot` 
   <details><summary>To assign label to disk do the command::</summary> 
  
 `yc compute disk add-labels --name=init-test-disk --labels=snapshot=1`
      
```json 
{
  "id": "fhmhsdpqauu4vasm5tsl",
  "folder_id": "b1gul7j7tkf53j6imep0",
  "created_at": "2020-08-19T15:51:36Z",
  "name": "init-test",
  "labels": 
         {
           "snapshot": "1"
         },
  "type_id": "network-hdd",
  "zone_id": "ru-central1-a",
  "size": "2445983875072",
  "block_size": "4096",
  "product_ids": [
    "f2e714m5slsflaoji565"
  ],
  "status": "READY",
  "source_snapshot_id": "fd8b2e4op5qmj27p2s79",
  "disk_placement_policy": 
  {

  }
}
```    
</details> 
   1.5 Fill in`SERVICE_ACCOUNT_ID=` use id of SA that  you've created earlier
    
   1.6 Fill in`TTL=` in Snapshot Time To Live in seconds. 1 week = 60*60*24*7 = 604800 will be TTL=604800
    
   1.7 Fill in `CRATE_CRON=` schedule for snapshot creation and `DELETE_CRON=` schedule for  removing expired snapshots. Both should be filled in in AWS-format            https://cloud.yandex.com/docs/functions/concepts/trigger/timer
    
   1.8 QUEUE_URL и QUEUE_ARN will be autopopulated, leave them empty.

### Deployment

2. `./script/create.sh` Launch this script first, it will create 3 functions, message queue and 3 triggers

3. `./script/deploy.sh` Launch this script next to upload the function code to bucket, and activate them

#### Windows

You can use mingw (git bash).

To install it first do:

1. Download and insstall [GnuZip](http://gnuwin32.sourceforge.net/packages/zip.htm)

2. Add `GnuZip` to PATH.

You will need administrator account to do that.
