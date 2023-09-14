package eventing

import (
	"context"
	"fmt"
	"os"
	"time"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	brokerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker"
	triggerinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/trigger"
	parallelinformer "knative.dev/eventing/pkg/client/injection/informers/flows/v1/parallel"
	sequenceinformer "knative.dev/eventing/pkg/client/injection/informers/flows/v1/sequence"
	channelinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/channel"
	imcinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel"
	subscriptioninformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/subscription"
	apiserversourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/apiserversource"
	containersourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/containersource"
	pingsourceinformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/pingsource"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/sources/v1/sinkbinding"
	eventingv1 "knative.dev/eventing/pkg/client/listers/eventing/v1"
	flowsv1 "knative.dev/eventing/pkg/client/listers/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/client/listers/messaging/v1"
	sourcesv1 "knative.dev/eventing/pkg/client/listers/sources/v1"
	"knative.dev/operator/pkg/apis/operator/base"
	operatorv1beta1 "knative.dev/operator/pkg/apis/operator/v1beta1"
	operator "knative.dev/operator/pkg/reconciler/common"
	operatorcommon "knative.dev/operator/pkg/reconciler/common"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"

	knativeeventinginformer "knative.dev/operator/pkg/client/injection/informers/operator/v1beta1/knativeeventing"

	"github.com/openshift-knative/serverless-operator/openshift-knative-operator/pkg/common"
	"github.com/openshift-knative/serverless-operator/openshift-knative-operator/pkg/monitoring"
	"github.com/openshift-knative/serverless-operator/pkg/istio/eventingistio"
)

const (
	requiredNsEnvName = "REQUIRED_EVENTING_NAMESPACE"
	defaultReplicas   = 2
)

func init() {
	applyKnativeEventingCRDs()
}

// NewExtension creates a new extension for a Knative Eventing controller.
func NewExtension(ctx context.Context, impl *controller.Impl) operator.Extension {

	globalResync := func(interface{}) {
		impl.GlobalResync(knativeeventinginformer.Get(ctx).Informer())
	}

	e := &extension{
		kubeclient: kubeclient.Get(ctx),

		brokerLister:  brokerinformer.Get(ctx).Lister(),
		triggerLister: triggerinformer.Get(ctx).Lister(),

		channelLister:      channelinformer.Get(ctx).Lister(),
		imcLister:          imcinformer.Get(ctx).Lister(),
		subscriptionLister: subscriptioninformer.Get(ctx).Lister(),

		apiServerSourceLister: apiserversourceinformer.Get(ctx).Lister(),
		pingSourceLister:      pingsourceinformer.Get(ctx).Lister(),
		containerSourceLister: containersourceinformer.Get(ctx).Lister(),
		sinkBindingLister:     sinkbindinginformer.Get(ctx).Lister(),

		parallelLister: parallelinformer.Get(ctx).Lister(),
		sequenceLister: sequenceinformer.Get(ctx).Lister(),
	}

	brokerinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	triggerinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))

	channelinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	imcinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	subscriptioninformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))

	apiserversourceinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	pingsourceinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	containersourceinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	sinkbindinginformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))

	sequenceinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))
	parallelinformer.Get(ctx).Informer().AddEventHandler(handleAddDelete(globalResync))

	if err := NewDefaultInstaller(ctx); err != nil {
		panic(fmt.Errorf("failed to create default installer: %v", err))
	}

	return e
}

type extension struct {
	kubeclient kubernetes.Interface

	brokerLister  eventingv1.BrokerLister
	triggerLister eventingv1.TriggerLister

	channelLister      messagingv1.ChannelLister
	imcLister          messagingv1.InMemoryChannelLister
	subscriptionLister messagingv1.SubscriptionLister

	apiServerSourceLister sourcesv1.ApiServerSourceLister
	pingSourceLister      sourcesv1.PingSourceLister
	sinkBindingLister     sourcesv1.SinkBindingLister
	containerSourceLister sourcesv1.ContainerSourceLister

	parallelLister flowsv1.ParallelLister
	sequenceLister flowsv1.SequenceLister
}

func (e *extension) Manifests(ke base.KComponent) ([]mf.Manifest, error) {
	m, err := monitoring.GetEventingMonitoringPlatformManifests(ke)
	if err != nil {
		return m, err
	}
	p, err := eventingistio.GetServiceMeshNetworkPolicy()
	if err != nil {
		return nil, err
	}
	if enabled := eventingistio.IsEnabled(ke.GetSpec().GetConfig()); enabled {
		m = append(m, p)
	}
	return m, nil
}

func (e *extension) Transformers(ke base.KComponent) []mf.Transformer {
	tf := []mf.Transformer{
		common.InjectCommonLabelIntoNamespace(),
		common.VersionedJobNameTransform(),
		common.InjectCommonEnvironment(),
	}
	tf = append(tf, monitoring.GetEventingTransformers(ke)...)
	return append(tf, common.DeprecatedAPIsTranformers(e.kubeclient.Discovery())...)
}

func (e *extension) Reconcile(ctx context.Context, comp base.KComponent) error {
	ke := comp.(*operatorv1beta1.KnativeEventing)

	requiredNs := os.Getenv(requiredNsEnvName)
	if requiredNs != "" && ke.Namespace != requiredNs {
		ke.Status.MarkInstallFailed(fmt.Sprintf("Knative Eventing must be installed into the namespace %q", requiredNs))
		return controller.NewPermanentError(fmt.Errorf("deployed Knative Eventing into unsupported namespace %q", ke.Namespace))
	}

	// Override images.
	// TODO(SRVCOM-1069): Rethink overriding behavior and/or error surfacing.
	images := common.ImageMapFromEnvironment(os.Environ())
	ke.Spec.Registry.Override = images
	ke.Spec.Registry.Default = images["default"]

	// Ensure webhook has 1G of memory.
	common.EnsureContainerMemoryLimit(&ke.Spec.CommonSpec, "eventing-webhook", resource.MustParse("1024Mi"))

	// SRVKE-500: Ensure we set the SinkBindingSelectionMode to inclusion
	if ke.Spec.SinkBindingSelectionMode == "" {
		ke.Spec.SinkBindingSelectionMode = "inclusion"
	}

	if !eventingistio.IsEnabled(ke.GetSpec().GetConfig()) {
		eventingistio.ScaleIstioController(requiredNs, ke, 0)
	} else {
		eventingistio.ScaleIstioController(requiredNs, ke, 1)
	}

	if err := e.AutoscaleComponents(ctx, ke); err != nil {
		return fmt.Errorf("failed to scale components: %w", err)
	}

	return monitoring.ReconcileMonitoringForEventing(ctx, e.kubeclient, ke)
}

func (e *extension) Finalize(context.Context, base.KComponent) error {
	return nil
}

func handleAddDelete(f func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    f,
		DeleteFunc: f,
	}
}

func applyKnativeEventingCRDs() {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil).ClientConfig()
	if err != nil {
		panic("Failed to create k8s client config: " + err.Error())
	}
	mfclient, err := mfc.NewClient(cfg)
	if err != nil {
		panic("Failed to create client: " + err.Error())
	}

	version := operatorcommon.LatestRelease(&operatorv1beta1.KnativeEventing{})

	path := fmt.Sprintf("%s/%s/1-eventing-crds.yaml", "/var/run/ko/knative-eventing", version)
	manifest, err := mf.ManifestFrom(mf.Path(path), mf.UseClient(mfclient))
	if err != nil {
		panic("Failed to create manifests from " + path + ": " + err.Error())
	}

	var lastErr error
	err = wait.PollImmediate(2*time.Second, 2*time.Minute, func() (bool, error) {
		if err := manifest.Apply(); err != nil {
			lastErr = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		panic(lastErr)
	}
}
