package scenario

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/lease"
	"k8s.io/apimachinery/pkg/util/wait"
)

const clusterLeasePrefix = "cluster-lifecycle-v1"
const recoveryNodeGroupMetadata = "recovery_node_group"

type clusterLease struct {
	ctx      context.Context
	cancel   context.CancelFunc
	blob     *blockblob.Client
	client   *lease.BlobClient
	done     chan struct{}
	renewErr error
	nodeName string
}

func clusterCoordinationScope() string {
	return config.Config.BlobStorageAccountURL() + "/" + config.Config.BlobContainer + "/" + clusterLeasePrefix
}

func acquireClusterLease(ctx context.Context, parentName string) (*clusterLease, error) {
	defer logging.LogStepf(ctx, "acquire cluster lifecycle lease for %s", parentName)()
	if config.Azure.Blob == nil {
		return nil, fmt.Errorf("cluster lifecycle coordination requires the E2E blob client")
	}
	client := config.Azure.Blob.ServiceClient().NewContainerClient(config.Config.BlobContainer).
		NewBlockBlobClient(clusterLeasePrefix + "/" + parentName)
	_, err := client.UploadBuffer(ctx, nil, &blockblob.UploadBufferOptions{
		AccessConditions: &blob.AccessConditions{ModifiedAccessConditions: &blob.ModifiedAccessConditions{
			IfNoneMatch: to.Ptr(azcore.ETagAny),
		}},
	})
	if err != nil && !bloberror.HasCode(err, bloberror.BlobAlreadyExists, bloberror.ConditionNotMet, bloberror.LeaseIDMissing) {
		return nil, fmt.Errorf("creating cluster lifecycle blob: %w", err)
	}
	leaseClient, err := lease.NewBlobClient(client, nil)
	if err != nil {
		return nil, fmt.Errorf("creating cluster lifecycle lease client: %w", err)
	}
	err = wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		_, err := leaseClient.AcquireLease(ctx, 60, nil)
		if bloberror.HasCode(err, bloberror.LeaseAlreadyPresent) {
			return false, nil
		}
		return err == nil, err
	})
	if err != nil {
		return nil, fmt.Errorf("acquiring cluster lifecycle lease: %w", err)
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	lock := &clusterLease{ctx: leaseCtx, cancel: cancel, blob: client, client: leaseClient, done: make(chan struct{})}
	go lock.renew()
	properties, err := client.GetProperties(leaseCtx, &blob.GetPropertiesOptions{AccessConditions: lock.accessConditions()})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("reading cluster recovery marker: %w", err), lock.close())
	}
	for name, nodeName := range properties.Metadata {
		if strings.EqualFold(name, recoveryNodeGroupMetadata) {
			if nodeName == nil || *nodeName == "" {
				return nil, errors.Join(fmt.Errorf("cluster recovery marker has no node resource group"), lock.close())
			}
			lock.nodeName = *nodeName
		}
	}
	return lock, nil
}

func (l *clusterLease) accessConditions() *blob.AccessConditions {
	return &blob.AccessConditions{LeaseAccessConditions: &lease.AccessConditions{LeaseID: l.client.LeaseID()}}
}

func (l *clusterLease) setRecovery(ctx context.Context, nodeName string) error {
	metadata := map[string]*string{}
	if nodeName != "" {
		metadata[recoveryNodeGroupMetadata] = to.Ptr(nodeName)
	}
	if _, err := l.blob.SetMetadata(ctx, metadata, &blob.SetMetadataOptions{AccessConditions: l.accessConditions()}); err != nil {
		return fmt.Errorf("updating cluster recovery marker: %w", err)
	}
	l.nodeName = nodeName
	return nil
}

func (l *clusterLease) renew() {
	defer close(l.done)
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			// Cancel ARM work well before the 60-second lease can expire.
			ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
			_, err := l.client.RenewLease(ctx, nil)
			cancel()
			if err != nil {
				if l.ctx.Err() == nil {
					l.renewErr = fmt.Errorf("renewing cluster lifecycle lease: %w", err)
					l.cancel()
				}
				return
			}
		}
	}
}

func (l *clusterLease) close() error {
	l.cancel()
	<-l.done
	ctx, cancel := context.WithTimeout(context.WithoutCancel(l.ctx), 10*time.Second)
	defer cancel()
	_, err := l.client.ReleaseLease(ctx, nil)
	if err != nil {
		err = fmt.Errorf("releasing cluster lifecycle lease: %w", err)
	}
	return errors.Join(l.renewErr, err)
}
